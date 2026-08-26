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
	urlTemplate = "http://201.34.132.118:3000/api/connections/{uuid}/config/"
	configPath  = "/etc/xray/proxy.json"
	routingPath = "/etc/xray/routing.json"
	routingUrl = "http://201.34.132.118:3000/api/connections/routingconfig"
)

func fetchData(uuid string) ([]byte, error) {
	url := strings.Replace(urlTemplate, "{uuid}", strings.TrimSpace(uuid), -1)
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

func fetchRouting() ([]byte, error){
	url := routingUrl
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

	return string(uuidBytes), nil
}

func main() {
	uuid, err := getUuid()

	if err != nil || uuid == "" {
		log.Printf("Ошибка получения id устройства")
		os.Exit(1)
	}

	// Ошибки двух загрузок держатся раздельно: раньше вторая перетирала первую,
	// и недоступный routing выглядел как успех. Тогда newRouting был пустым, не
	// совпадал с файлом на диске — и роутер записывал пустой routing.json, после
	// чего перезапускал xray. Устройство теряло маршрутизацию из-за того, что
	// сервер минуту не отвечал.
	newRouting, routingErr := fetchRouting()

	newConfig, configErr := fetchData(uuid)

	if configErr != nil {
		log.Printf("Ошибка загрузки конфигурации")
		log.Printf("%v", configErr)
	} else {
		writeIfChanged(configPath, newConfig)
	}

	if routingErr != nil {
		log.Printf("Ошибка загрузки маршрутизации")
		log.Printf("%v", routingErr)
	} else {
		writeIfChanged(routingPath, newRouting)
	}

	reportLiveness(uuid)
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
