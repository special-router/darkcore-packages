package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/special-router/dcvpnupd/xray/observatory/command"
)

const (
	observatoryAddr     = "127.0.0.1:8081"
	livenessURLTemplate = "http://201.34.132.118:3000/api/connections/{uuid}/liveness"
	livenessTimeout     = 4 * time.Second
	proxyTagPrefix      = "proxy"
)

// Observation описывает измерение живости одного outbound'а, взятое из
// burstObservatory самого роутера, а не из наших дата-центровых серверов.
type Observation struct {
	Tag          string `json:"tag"`
	Alive        bool   `json:"alive"`
	DelayMs      int64  `json:"delayMs"`
	LastSeenTime int64  `json:"lastSeenTime"`
	LastTryTime  int64  `json:"lastTryTime"`
}

type livenessPayload struct {
	Observations []Observation `json:"observations"`
}

func telemetryEnabled() bool {
	out, err := exec.Command("uci", "-q", "get", "darkcore.main.telemetry_enabled").Output()
	if err != nil {
		return false
	}

	switch strings.TrimSpace(string(out)) {
	case "1", "true":
		return true
	default:
		return false
	}
}

func getProvisionToken() (string, error) {
	out, err := exec.Command("uci", "-q", "get", "darkcore.provision.token").Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

func collectObservations(ctx context.Context) ([]Observation, error) {
	conn, err := grpc.NewClient(observatoryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := command.NewObservatoryServiceClient(conn)

	resp, err := client.GetOutboundStatus(ctx, &command.GetOutboundStatusRequest{})
	if err != nil {
		return nil, err
	}

	var observations []Observation
	for _, status := range resp.GetStatus().GetStatus() {
		if !strings.HasPrefix(status.GetOutboundTag(), proxyTagPrefix) {
			continue
		}

		observations = append(observations, Observation{
			Tag:          status.GetOutboundTag(),
			Alive:        status.GetAlive(),
			DelayMs:      status.GetDelay(),
			LastSeenTime: status.GetLastSeenTime(),
			LastTryTime:  status.GetLastTryTime(),
		})
	}

	return observations, nil
}

func postLiveness(ctx context.Context, uuid, token string, payload []byte) error {
	url := strings.Replace(livenessURLTemplate, "{uuid}", uuid, -1)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Darkcore-Token", token)

	client := &http.Client{Timeout: livenessTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return APIError{
			StatusCode: resp.StatusCode,
			Message:    resp.Status,
			Details:    string(body),
		}
	}

	return nil
}

// reportLiveness — best-effort стадия: собирает результаты burstObservatory
// самого xray и пересылает их на сервер как измерения живости из сети
// устройства. Любая ошибка здесь только логируется и не влияет на код
// возврата dcvpnupd — этим отличается от fetchData/fetchRouting выше.
func reportLiveness(uuid string) {
	uuid = strings.TrimSpace(uuid)

	if !telemetryEnabled() {
		return
	}

	token, err := getProvisionToken()
	if err != nil || token == "" {
		log.Printf("Телеметрия: не удалось получить токен, пропуск")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), livenessTimeout)
	defer cancel()

	observations, err := collectObservations(ctx)
	if err != nil {
		log.Printf("Телеметрия: ошибка обращения к observatory API: %v", err)
		return
	}

	payload, err := json.Marshal(livenessPayload{Observations: observations})
	if err != nil {
		log.Printf("Телеметрия: ошибка сериализации: %v", err)
		return
	}

	postCtx, postCancel := context.WithTimeout(context.Background(), livenessTimeout)
	defer postCancel()

	if err := postLiveness(postCtx, uuid, token, payload); err != nil {
		log.Printf("Телеметрия: ошибка отправки: %v", err)
		return
	}

	log.Printf("Телеметрия: отправлено %d наблюдений", len(observations))
}
