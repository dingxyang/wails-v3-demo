package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	providerAuto      = "auto"
	providerMicrosoft = "microsoft"
	providerGoogle    = "google"

	// Microsoft retired the old free token endpoint (edge.microsoft.com/translate/auth)
	// in August 2026. Edge's built-in translator now posts straight to translatetext,
	// which needs no credentials, just a User-Agent.
	microsoftTranslateURL = "https://edge.microsoft.com/translate/translatetext"
	microsoftUserAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36 Edg/113.0.1774.35"

	googleTranslateURL = "https://translate.google.com/translate_a/single"
)

// Language is an option shown in the source/target selectors.
type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// TranslateResult is returned to the frontend after a successful translation.
type TranslateResult struct {
	Text           string `json:"text"`
	DetectedSource string `json:"detectedSource"`
	Provider       string `json:"provider"`
}

// TranslationService provides keyless translation through the free endpoints
// used by browser translation extensions.
type TranslationService struct {
	client *http.Client
}

func NewTranslationService() *TranslationService {
	return &TranslationService{client: &http.Client{Timeout: 25 * time.Second}}
}

var supportedLanguages = []Language{
	{Code: "zh-CN", Name: "简体中文"},
	{Code: "zh-TW", Name: "繁体中文"},
	{Code: "en", Name: "English"},
	{Code: "ja", Name: "日本語"},
	{Code: "ko", Name: "한국어"},
	{Code: "fr", Name: "Français"},
	{Code: "de", Name: "Deutsch"},
	{Code: "es", Name: "Español"},
	{Code: "ru", Name: "Русский"},
	{Code: "pt", Name: "Português"},
	{Code: "it", Name: "Italiano"},
	{Code: "ar", Name: "العربية"},
	{Code: "th", Name: "ไทย"},
	{Code: "vi", Name: "Tiếng Việt"},
	{Code: "id", Name: "Bahasa Indonesia"},
	{Code: "tr", Name: "Türkçe"},
	{Code: "nl", Name: "Nederlands"},
	{Code: "pl", Name: "Polski"},
}

// Languages exposes the target list so the frontend and backend share one source.
func (s *TranslationService) Languages() []Language {
	return supportedLanguages
}

// Translate translates text using the selected provider. "auto" tries
// Microsoft first (reachable in mainland China) and falls back to Google.
func (s *TranslationService) Translate(text, source, target, provider string) (TranslateResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return TranslateResult{}, errors.New("请输入要翻译的文本")
	}
	if target == "" {
		target = "zh-CN"
	}
	if source == "" {
		source = "auto"
	}

	ctx := context.Background()
	switch provider {
	case providerGoogle:
		return s.translateWithGoogle(ctx, text, source, target)
	case providerMicrosoft:
		return s.translateWithMicrosoft(ctx, text, source, target)
	case "", providerAuto:
		result, msErr := s.translateWithMicrosoft(ctx, text, source, target)
		if msErr == nil {
			return result, nil
		}
		result, googleErr := s.translateWithGoogle(ctx, text, source, target)
		if googleErr == nil {
			return result, nil
		}
		return TranslateResult{}, fmt.Errorf("微软翻译失败：%v；谷歌翻译失败：%v", msErr, googleErr)
	default:
		return TranslateResult{}, fmt.Errorf("不支持的翻译服务：%s", provider)
	}
}

func (s *TranslationService) translateWithMicrosoft(ctx context.Context, text, source, target string) (TranslateResult, error) {
	query := url.Values{}
	query.Set("to", microsoftLanguageCode(target))
	if !isAutoLanguage(source) {
		query.Set("from", microsoftLanguageCode(source))
	}
	query.Set("isEnterpriseClient", "false")

	payload, err := json.Marshal([]string{text})
	if err != nil {
		return TranslateResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		microsoftTranslateURL+"?"+query.Encode(), strings.NewReader(string(payload)))
	if err != nil {
		return TranslateResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", microsoftUserAgent)

	var response []struct {
		DetectedLanguage struct {
			Language string `json:"language"`
		} `json:"detectedLanguage"`
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := s.doJSON(req, &response); err != nil {
		return TranslateResult{}, fmt.Errorf("微软翻译：%w", err)
	}
	if len(response) == 0 || len(response[0].Translations) == 0 {
		return TranslateResult{}, errors.New("微软翻译返回了空结果")
	}

	return TranslateResult{
		Text:           response[0].Translations[0].Text,
		DetectedSource: response[0].DetectedLanguage.Language,
		Provider:       providerMicrosoft,
	}, nil
}

func (s *TranslationService) translateWithGoogle(ctx context.Context, text, source, target string) (TranslateResult, error) {
	form := url.Values{}
	form.Set("client", "gtx")
	form.Set("sl", "auto")
	if !isAutoLanguage(source) {
		form.Set("sl", source)
	}
	form.Set("tl", target)
	form.Set("dt", "t")
	form.Set("q", text)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		googleTranslateURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TranslateResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var response []json.RawMessage
	if err := s.doJSON(req, &response); err != nil {
		return TranslateResult{}, fmt.Errorf("谷歌翻译：%w", err)
	}
	if len(response) == 0 {
		return TranslateResult{}, errors.New("谷歌翻译返回了空结果")
	}

	// The response is a deeply nested array: [[[translated, original, ...], ...], ..., detected]
	var segments []json.RawMessage
	if err := json.Unmarshal(response[0], &segments); err != nil {
		return TranslateResult{}, fmt.Errorf("谷歌翻译响应格式异常：%w", err)
	}
	var translated strings.Builder
	for _, segment := range segments {
		var parts []json.RawMessage
		if err := json.Unmarshal(segment, &parts); err != nil || len(parts) == 0 {
			continue
		}
		var part string
		if err := json.Unmarshal(parts[0], &part); err == nil {
			translated.WriteString(part)
		}
	}
	if translated.Len() == 0 {
		return TranslateResult{}, errors.New("谷歌翻译返回了空结果")
	}

	detected := ""
	if len(response) > 2 {
		_ = json.Unmarshal(response[2], &detected)
	}
	return TranslateResult{
		Text:           translated.String(),
		DetectedSource: detected,
		Provider:       providerGoogle,
	}, nil
}

// doJSON executes req, checks the HTTP status and decodes the JSON body.
func (s *TranslationService) doJSON(req *http.Request, target any) error {
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d：%s", resp.StatusCode, snippet(body))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("无法解析响应：%w", err)
	}
	return nil
}

func isAutoLanguage(language string) bool {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "", "auto", "auto-detect":
		return true
	default:
		return false
	}
}

func microsoftLanguageCode(language string) string {
	switch language {
	case "zh-CN":
		return "zh-Hans"
	case "zh-TW":
		return "zh-Hant"
	default:
		return language
	}
}

func snippet(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 300 {
		text = text[:300] + "…"
	}
	return text
}
