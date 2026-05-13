package joiner

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"whitelist-bypass-iran/relay/common"
)

const baleWSURL = "wss://meet.bale.ai/ws"

type baleAnonConfig struct {
	WSURL string
	Token string
}

func baleHttpGet(client *http.Client, endpoint, origin string) ([]byte, error) {
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("User-Agent", common.UserAgent)
	if origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Referer", origin+"/")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		max := 120
		if len(body) < max {
			max = len(body)
		}
		return nil, fmt.Errorf("status %d body=%q", resp.StatusCode, string(body)[:max])
	}
	return body, nil
}

func baleLooksLikeJWT(s string) bool {
	if !strings.HasPrefix(s, "eyJ") {
		return false
	}
	return strings.Count(s, ".") == 2
}

func baleFetchAnonConfig(client *http.Client, logFn func(string, ...any)) (baleAnonConfig, error) {
	cfg := baleAnonConfig{WSURL: baleWSURL}

	var token string
	for attempt := 1; attempt <= 5; attempt++ {
		body, terr := baleHttpGet(client, "https://meet.bale.ai/token", "https://meet.bale.ai")
		if terr != nil {
			logFn("[config] /token attempt %d: %v", attempt, terr)
			continue
		}
		t := strings.TrimSpace(string(body))
		if baleLooksLikeJWT(t) {
			token = t
			break
		}
		logFn("[config] /token attempt %d: non-jwt response (%d bytes), retrying", attempt, len(body))
	}
	if token == "" {
		return cfg, fmt.Errorf("could not obtain valid /token after 5 attempts")
	}
	cfg.Token = token

	logFn("[config] ws=%s tokenLen=%d", cfg.WSURL, len(cfg.Token))
	return cfg, nil
}
