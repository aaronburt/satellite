package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidEnvelope    = errors.New("invalid or unrecognized configuration envelope")
	ErrInvalidPassphrase  = errors.New("invalid passphrase or corrupted configuration")
	ErrPassphraseRequired = errors.New("configuration is encrypted; passphrase required")
)

const EnvelopeFormat = "satellite-config"

type ImportOptions struct {
	Passphrase  string
	ResetNodeID bool
	ResetAPIKey bool
}

type ExportData struct {
	NodeID       string        `json:"node_id,omitempty"`
	MQTT         MQTTConfig    `json:"mqtt"`
	Expose       ExposeConfig  `json:"expose"`
	IntervalSec  int           `json:"interval_sec"`
	Port         int           `json:"port,omitempty"`
	WebUIPort    int           `json:"webui_port,omitempty"`
	BindAddress  string        `json:"bind_address,omitempty"`
	JSONEnabled  bool          `json:"json_enabled,omitempty"`
	APIKey       string        `json:"api_key,omitempty"`
	Webhook      WebhookConfig `json:"webhook"`
	CheckUpdates bool          `json:"check_updates"`
	UpdateRepo   string        `json:"update_repo,omitempty"`
}

type ExportEnvelope struct {
	Format     string      `json:"format"`
	AppVersion string      `json:"app_version"`
	ExportedAt int64       `json:"exported_at"`
	Encrypted  bool        `json:"encrypted"`
	KDF        string      `json:"kdf,omitempty"`
	Iterations int         `json:"iterations,omitempty"`
	Salt       string      `json:"salt,omitempty"`
	Nonce      string      `json:"nonce,omitempty"`
	Payload    string      `json:"payload,omitempty"`
	Data       *ExportData `json:"data,omitempty"`
}

type ImportResult struct {
	Config           Config
	SourceAppVersion string
	IsNewerVersion   bool
}

func parseVersionComponents(v string) (int, int, int) {
	clean := strings.TrimSpace(v)
	clean = strings.TrimPrefix(clean, "v")
	clean = strings.TrimPrefix(clean, "V")
	if idx := strings.IndexAny(clean, "-+"); idx != -1 {
		clean = clean[:idx]
	}
	parts := strings.Split(clean, ".")
	var major, minor, patch int
	if len(parts) > 0 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) > 2 {
		patch, _ = strconv.Atoi(parts[2])
	}
	return major, minor, patch
}

func isVersionNewer(v1, v2 string) bool {
	maj1, min1, pat1 := parseVersionComponents(v1)
	maj2, min2, pat2 := parseVersionComponents(v2)
	if maj1 != maj2 {
		return maj1 > maj2
	}
	if min1 != min2 {
		return min1 > min2
	}
	return pat1 > pat2
}

func ExportConfig(cfg Config, passphrase string) ([]byte, error) {
	exportData := ExportData{
		NodeID:       cfg.NodeID,
		MQTT:         cfg.MQTT,
		Expose:       cfg.Expose,
		IntervalSec:  cfg.IntervalSec,
		Port:         cfg.Port,
		WebUIPort:    cfg.WebUIPort,
		BindAddress:  cfg.BindAddress,
		JSONEnabled:  cfg.JSONEnabled,
		APIKey:       cfg.APIKey,
		Webhook:      cfg.Webhook,
		CheckUpdates: cfg.CheckUpdates,
		UpdateRepo:   cfg.UpdateRepo,
	}

	trimmedPass := strings.TrimSpace(passphrase)
	if trimmedPass == "" {
		env := ExportEnvelope{
			Format:     EnvelopeFormat,
			AppVersion: Version,
			ExportedAt: time.Now().Unix(),
			Encrypted:  false,
			Data:       &exportData,
		}
		return json.MarshalIndent(env, "", "  ")
	}

	rawJSON, err := json.Marshal(exportData)
	if err != nil {
		return nil, err
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	const iterations = 100000
	key, err := pbkdf2.Key(sha256.New, trimmedPass, salt, iterations, 32)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, rawJSON, []byte(EnvelopeFormat))

	env := ExportEnvelope{
		Format:     EnvelopeFormat,
		AppVersion: Version,
		ExportedAt: time.Now().Unix(),
		Encrypted:  true,
		KDF:        "pbkdf2-sha256",
		Iterations: iterations,
		Salt:       hex.EncodeToString(salt),
		Nonce:      hex.EncodeToString(nonce),
		Payload:    base64.StdEncoding.EncodeToString(ciphertext),
	}

	return json.MarshalIndent(env, "", "  ")
}

func ImportConfig(raw []byte, opts ImportOptions) (ImportResult, error) {
	var env ExportEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ImportResult{}, fmt.Errorf("%w: %v", ErrInvalidEnvelope, err)
	}

	if env.Format != EnvelopeFormat {
		return ImportResult{}, ErrInvalidEnvelope
	}

	var data ExportData
	if env.Encrypted {
		trimmedPass := strings.TrimSpace(opts.Passphrase)
		if trimmedPass == "" {
			return ImportResult{}, ErrPassphraseRequired
		}

		salt, err := hex.DecodeString(env.Salt)
		if err != nil || len(salt) == 0 {
			return ImportResult{}, ErrInvalidEnvelope
		}

		nonce, err := hex.DecodeString(env.Nonce)
		if err != nil || len(nonce) == 0 {
			return ImportResult{}, ErrInvalidEnvelope
		}

		ciphertext, err := base64.StdEncoding.DecodeString(env.Payload)
		if err != nil || len(ciphertext) == 0 {
			return ImportResult{}, ErrInvalidEnvelope
		}

		iters := env.Iterations
		if iters <= 0 {
			iters = 100000
		}

		key, err := pbkdf2.Key(sha256.New, trimmedPass, salt, iters, 32)
		if err != nil {
			return ImportResult{}, ErrInvalidPassphrase
		}

		block, err := aes.NewCipher(key)
		if err != nil {
			return ImportResult{}, err
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return ImportResult{}, err
		}

		if len(nonce) != gcm.NonceSize() {
			return ImportResult{}, ErrInvalidEnvelope
		}

		plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(EnvelopeFormat))
		if err != nil {
			return ImportResult{}, ErrInvalidPassphrase
		}

		if err := json.Unmarshal(plaintext, &data); err != nil {
			return ImportResult{}, fmt.Errorf("%w: %v", ErrInvalidEnvelope, err)
		}
	} else {
		if env.Data == nil {
			return ImportResult{}, ErrInvalidEnvelope
		}
		data = *env.Data
	}

	def := DefaultConfig()
	cfg := Config{
		NodeID:       data.NodeID,
		MQTT:         data.MQTT,
		Expose:       data.Expose,
		IntervalSec:  data.IntervalSec,
		Port:         data.Port,
		WebUIPort:    data.WebUIPort,
		BindAddress:  data.BindAddress,
		JSONEnabled:  data.JSONEnabled,
		APIKey:       data.APIKey,
		Webhook:      data.Webhook,
		CheckUpdates: data.CheckUpdates,
		UpdateRepo:   data.UpdateRepo,
	}

	if cfg.IntervalSec <= 0 {
		cfg.IntervalSec = def.IntervalSec
	}
	if cfg.BindAddress == "" {
		cfg.BindAddress = def.BindAddress
	}
	if cfg.UpdateRepo == "" {
		cfg.UpdateRepo = def.UpdateRepo
	}
	if cfg.MQTT.TopicPrefix == "" {
		cfg.MQTT.TopicPrefix = def.MQTT.TopicPrefix
	}
	if cfg.Webhook.MinLevel == "" {
		cfg.Webhook.MinLevel = def.Webhook.MinLevel
	}

	if opts.ResetNodeID || cfg.NodeID == "" {
		cfg.NodeID = def.NodeID
		cfg.MQTT.ClientID = "satellite-" + def.NodeID
	}
	if opts.ResetAPIKey || (cfg.JSONEnabled && cfg.APIKey == "") {
		cfg.APIKey = GenerateAPIKey()
	}

	return ImportResult{
		Config:           cfg,
		SourceAppVersion: env.AppVersion,
		IsNewerVersion:   isVersionNewer(env.AppVersion, Version),
	}, nil
}
