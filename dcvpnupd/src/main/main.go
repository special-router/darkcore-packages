package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"fmt"
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
	defaultAPIBase = "https://sub.special-wifi.ru"
	configPath     = "/etc/xray/proxy.json"
)

// getAPIBase: сперва UCI darkcore.main.api_base, иначе вкомпилированный
// дефолт. Партию плат можно перевести на другой сервер одним `uci set`,
// без пересборки dcvpnupd.
func getAPIBase() string {
	out, err := exec.Command("uci", "-q", "get", "darkcore.main.api_base").Output()
	if err == nil {
		if base := strings.TrimSpace(string(out)); base != "" {
			return base
		}
	}
	return defaultAPIBase
}

func fetchConfig(base, uuid string) ([]byte, error) {
	url := strings.TrimSuffix(base, "/") + "/api/v1/vpn/box/" + uuid + "/config/"
	resp, err := http.Get(url)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, APIError{
			StatusCode: resp.StatusCode,
			Message:    resp.Status,
			Details:    string(body),
		}
	}

	return body, nil
}

func restartService() {
	cmd := exec.Command("service", "xray", "restart")
	if err := cmd.Run(); err != nil {
		log.Printf("Ошибка перезапуска xray: %v", err)
	} else {
		log.Println("Сервис xray перезапущен")
	}
}

func getUuid() (string, error) {
	uuidBytes, err := exec.Command("uci", "get", "darkcore.main.uuid").Output()

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(uuidBytes)), nil
}

func main() {
	uuid, err := getUuid()

	if err != nil || uuid == "" {
		log.Printf("Ошибка получения id устройства")
		os.Exit(1)
	}

	newConfig, configErr := fetchConfig(getAPIBase(), uuid)

	if configErr != nil {
		log.Printf("Ошибка загрузки конфигурации")
		log.Printf("%v", configErr)
		return
	}

	writeIfChanged(configPath, newConfig)
}

// writeIfChanged перезаписывает файл только осмысленным содержимым.
//
// Пустой ответ отбрасывается: единственное, чем он может стать на диске, —
// конфигурация, с которой xray не поднимется, а старая к тому моменту уже
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
