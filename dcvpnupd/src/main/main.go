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
	urlTemplate = "http://195.66.213.74:3000/api/connections/{uuid}/config/"
	configPath  = "/etc/xray/proxy.json"
	routingPath = "/etc/xray/routing.json"
	routingUrl = "http://195.66.213.74:3000/api/connections/routingconfig"
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

	newRouting, err := fetchRouting()

	newConfig, err := fetchData(uuid)

	if err != nil {
		log.Printf("Ошибка загрузки JSON")
		log.Printf("%v", err)
	} else {
		oldData, _ := os.ReadFile(configPath)
		oldRouing, _ := os.ReadFile(routingPath)

		if string(newConfig) != string(oldData) {
			if err := os.WriteFile(configPath, newConfig, 0644); err != nil {
				log.Printf("Ошибка записи файла: %v", err)
			} else {
				restartService()
			}
		} else {
			log.Println("Изменений нет")
		}

		if string(newRouting) != string(oldRouing) {
			if err := os.WriteFile(routingPath, newRouting, 0644); err != nil {
				log.Printf("Ошибка записи файла: %v", err)
			} else {
				restartService()
			}
		} else {
			log.Println("Изменений нет")
		}
	}
}
