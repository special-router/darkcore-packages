package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

const (
	// Базовый адрес backend по умолчанию. Переопределяется без пересборки
	// через `uci set darkcore.main.api_base=...` (см. getAPIBase).
	defaultAPIBase = "https://special-wifi.link"
	activatePath   = "/api/v1/vpn/router/activate/"

	// TODO(sing-box): провизорно, пока в darkcore-packages нет пакета
	// sing-box с собственным confdir/init-скриптом. Поправить, когда он
	// появится.
	configPath    = "/etc/sing-box/proxy.json"
	targetService = "sing-box"
)

type activateResponse struct {
	DeviceToken string `json:"device_token"`
	ConfigURL   string `json:"config_url"`
}

func uciGet(key string) string {
	out, err := exec.Command("uci", "-q", "get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func uciSet(key, value string) error {
	return exec.Command("uci", "set", key+"="+value).Run()
}

func uciCommit(config string) error {
	return exec.Command("uci", "commit", config).Run()
}

// getAPIBase: сперва UCI darkcore.main.api_base, иначе вкомпилированный
// дефолт. Партию плат можно перевести на другой сервер одним `uci set`,
// без пересборки dcvpnupd.
func getAPIBase() string {
	if base := uciGet("darkcore.main.api_base"); base != "" {
		return base
	}
	return defaultAPIBase
}

// activate меняет одноразовый код активации на device_token + config_url.
func activate(base, code string) (activateResponse, error) {
	url := strings.TrimSuffix(base, "/") + activatePath
	payload, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return activateResponse{}, err
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return activateResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return activateResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return activateResponse{}, APIError{StatusCode: resp.StatusCode, Message: resp.Status, Details: string(body)}
	}

	var parsed activateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return activateResponse{}, err
	}
	if parsed.DeviceToken == "" || parsed.ConfigURL == "" {
		return activateResponse{}, fmt.Errorf("активация вернула пустой device_token/config_url")
	}

	return parsed, nil
}

// ensureActivated отдаёт (device_token, config_url), уже сохранённые в UCI
// с прошлой активации. Если их ещё нет, но есть свежий одноразовый код —
// активируется через backend и сохраняет результат в UCI, чтобы код,
// который одноразовый, использовался только один раз.
func ensureActivated() (string, string, error) {
	token := uciGet("darkcore.main.device_token")
	configURL := uciGet("darkcore.main.config_url")
	if token != "" && configURL != "" {
		return token, configURL, nil
	}

	code := uciGet("darkcore.main.activation_code")
	if code == "" {
		return "", "", fmt.Errorf("устройство не активировано: нет ни device_token, ни activation_code")
	}

	result, err := activate(getAPIBase(), code)
	if err != nil {
		return "", "", fmt.Errorf("активация не удалась: %w", err)
	}

	if err := uciSet("darkcore.main.device_token", result.DeviceToken); err != nil {
		log.Printf("Не удалось сохранить device_token: %v", err)
	}
	if err := uciSet("darkcore.main.config_url", result.ConfigURL); err != nil {
		log.Printf("Не удалось сохранить config_url: %v", err)
	}
	if err := uciSet("darkcore.main.activation_code", ""); err != nil {
		log.Printf("Не удалось очистить activation_code: %v", err)
	}
	if err := uciCommit("darkcore"); err != nil {
		return "", "", fmt.Errorf("uci commit не удался: %w", err)
	}

	log.Println("Устройство активировано")
	return result.DeviceToken, result.ConfigURL, nil
}

func fetchConfig(configURL, deviceToken string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, configURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+deviceToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, APIError{StatusCode: resp.StatusCode, Message: resp.Status, Details: string(body)}
	}

	return body, nil
}

func restartService() {
	cmd := exec.Command("service", targetService, "restart")
	if err := cmd.Run(); err != nil {
		log.Printf("Ошибка перезапуска %s: %v", targetService, err)
	} else {
		log.Printf("Сервис %s перезапущен", targetService)
	}
}

func main() {
	deviceToken, configURL, err := ensureActivated()
	if err != nil {
		log.Printf("%v", err)
		os.Exit(1)
	}

	newConfig, err := fetchConfig(configURL, deviceToken)
	if err != nil {
		log.Printf("Ошибка загрузки конфигурации")
		log.Printf("%v", err)
		return
	}

	writeIfChanged(configPath, newConfig)
}

// writeIfChanged перезаписывает файл только осмысленным содержимым.
//
// Пустой ответ отбрасывается: единственное, чем он может стать на диске, —
// конфигурация, с которой сервис не поднимется, а старая к тому моменту уже
// затёрта. Сохранить прежнюю рабочую копию всегда лучше.
func writeIfChanged(path string, data []byte) {
	if len(data) == 0 {
		log.Printf("Пустой ответ для %s, файл не тронут", path)
		return
	}

	old, _ := os.ReadFile(path)

	if string(data) == string(old) {
		log.Println("Изменений нет")
		return
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("Ошибка записи файла: %v", err)
		return
	}

	restartService()
}
