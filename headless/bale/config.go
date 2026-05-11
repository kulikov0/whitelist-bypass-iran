package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"

	"whitelist-bypass-iran/relay/common"
)

type BaleConfig struct {
	APIVersion int64
	WSURL      string
}

func fetchConfig() (BaleConfig, error) {
	var cfg BaleConfig

	page, err := common.HttpGet("https://web.bale.ai/")
	if err != nil {
		return cfg, fmt.Errorf("fetch web.bale.ai: %w", err)
	}

	bundleRe := regexp.MustCompile(`/static/js/index\.[a-f0-9]+\.js`)
	bundlePath := bundleRe.FindString(string(page))
	if bundlePath == "" {
		return cfg, fmt.Errorf("index bundle not found in page")
	}
	bundleURL := "https://web.bale.ai" + bundlePath
	log.Printf("[config] bundle: %s", bundleURL)

	bundle, err := common.HttpGet(bundleURL)
	if err != nil {
		return cfg, fmt.Errorf("fetch bundle: %w", err)
	}

	wsRe := regexp.MustCompile(`"(wss://[a-zA-Z0-9.\-]+/ws/?)"`)
	if m := wsRe.FindSubmatch(bundle); m != nil {
		cfg.WSURL = string(m[1])
	} else {
		return cfg, fmt.Errorf("ws url not found in bundle")
	}

	apiRe := regexp.MustCompile(`Number\("(\d+)"\)`)
	if m := apiRe.FindSubmatch(bundle); m != nil {
		v, err := strconv.ParseInt(string(m[1]), 10, 64)
		if err != nil {
			return cfg, fmt.Errorf("parse apiVersion %q: %w", m[1], err)
		}
		cfg.APIVersion = v
	} else {
		return cfg, fmt.Errorf("apiVersion not found in bundle")
	}

	log.Printf("[config] ws=%s apiVersion=%d", cfg.WSURL, cfg.APIVersion)
	return cfg, nil
}
