package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

const (
	urlTemplate = "http://195.66.213.120:3000/api/connections/{uuid}/config/"
	configPath  = "/etc/xray/proxy.json"
)

func fetchData(uuid string) ([]byte, error) {
	log.Printf("%v", uuid)

	url := strings.Replace(urlTemplate, "{uuid}", strings.TrimSpace(uuid), -1)
	resp, err := http.Get(url)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
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

  log.Printf("hui: %v", uuid)

	newConfig, err := fetchData(uuid)

	if err != nil {
		log.Printf("Ошибка загрузки JSON")
		log.Printf("%v", err)
	} else {
		oldData, _ := os.ReadFile(configPath)

		if string(newConfig) != string(oldData) {
			if err := os.WriteFile(configPath, newConfig, 0644); err != nil {
				log.Printf("Ошибка записи файла: %v", err)
			} else {
				restartService()
			}
		} else {
			log.Println("Изменений нет")
		}
	}
}
