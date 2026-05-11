package joiner

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"whitelist-bypass-iran/relay/common"
)

type baleAnonConfig struct {
	APIVersion int64
	WSURL      string
	Token      string
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
	var cfg baleAnonConfig

	page, err := baleHttpGet(client, "https://meet.bale.ai/i/dummy", "")
	if err != nil {
		return cfg, fmt.Errorf("fetch meet.bale.ai page: %w", err)
	}

	bundleRe := regexp.MustCompile(`/static/js/index\.[a-f0-9]+\.js`)
	bundlePath := bundleRe.FindString(string(page))
	if bundlePath == "" {
		return cfg, fmt.Errorf("index bundle not found in meet page")
	}
	bundleURL := "https://meet.bale.ai" + bundlePath
	logFn("[config] bundle: %s", bundleURL)

	if _, err := baleHttpGet(client, bundleURL, ""); err != nil {
		return cfg, fmt.Errorf("fetch bundle: %w", err)
	}

	cfg.WSURL = "wss://meet.bale.ai/ws"
	cfg.APIVersion = 1

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

	logFn("[config] ws=%s apiVersion=%d tokenLen=%d", cfg.WSURL, cfg.APIVersion, len(cfg.Token))
	return cfg, nil
}

func baleExtractShareCode(joinLink string) string {
	s := strings.TrimSpace(joinLink)
	s = strings.TrimRight(s, "/")
	if i := strings.IndexByte(s, '?'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '#'); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	return s
}
