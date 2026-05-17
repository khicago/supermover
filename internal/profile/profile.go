package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/durable"
	"github.com/khicago/supermover/internal/transport"
)

const CurrentVersion = 1

type ConsistencyMode string

const (
	ConsistencyLive     ConsistencyMode = "live"
	ConsistencyStrict   ConsistencyMode = "strict"
	ConsistencySnapshot ConsistencyMode = "snapshot"
)

type DeleteMode string

const (
	DeleteModeIgnore DeleteMode = "ignore"
	DeleteModeRecord DeleteMode = "record"
	DeleteModePrune  DeleteMode = "prune"
)

type MetadataMode string

const (
	MetadataModeBasic    MetadataMode = "basic"
	MetadataModePreserve MetadataMode = "preserve"
)

type PrivacyMode string

const (
	PrivacyModePlaintext PrivacyMode = "plaintext"
	PrivacyModeRedacted  PrivacyMode = "redacted"
)

type Profile struct {
	Version              int               `json:"version"`
	ProfileID            string            `json:"profile_id"`
	Name                 string            `json:"name"`
	Roots                []Root            `json:"roots"`
	Include              []Rule            `json:"include,omitempty"`
	Exclude              []Rule            `json:"exclude,omitempty"`
	Consistency          ConsistencyMode   `json:"consistency"`
	DeletePolicy         DeletePolicy      `json:"delete_policy"`
	MetadataPolicy       MetadataPolicy    `json:"metadata_policy"`
	PrivacyPolicy        PrivacyPolicy     `json:"privacy_policy"`
	Target               TargetIdentity    `json:"target"`
	Network              *NetworkConfig    `json:"network,omitempty"`
	AgentKnowledge       AgentKnowledge    `json:"agent_knowledge"`
	SupplementalMetadata map[string]string `json:"supplemental_metadata,omitempty"`
}

type Root struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type Rule struct {
	Pattern string `json:"pattern"`
}

type DeletePolicy struct {
	Mode               DeleteMode `json:"mode"`
	RequireReview      bool       `json:"require_review"`
	RetentionDays      int        `json:"retention_days,omitempty"`
	AllowPhysicalPrune bool       `json:"allow_physical_prune,omitempty"`
}

type MetadataPolicy struct {
	Mode                 MetadataMode `json:"mode"`
	PreservePermissions  bool         `json:"preserve_permissions,omitempty"`
	PreserveModTime      bool         `json:"preserve_mod_time,omitempty"`
	PreserveExtendedAttr bool         `json:"preserve_extended_attr,omitempty"`
}

type PrivacyPolicy struct {
	Mode                    PrivacyMode `json:"mode"`
	TrafficLevel            int         `json:"traffic_level"`
	AllowPlaintextRestore   bool        `json:"allow_plaintext_restore"`
	AllowHiddenFiles        bool        `json:"allow_hidden_files,omitempty"`
	AllowSensitiveFilenames bool        `json:"allow_sensitive_filenames,omitempty"`
	PaddingBucketBytes      int         `json:"padding_bucket_bytes,omitempty"`
	BatchMaxBytes           int         `json:"batch_max_bytes,omitempty"`
	BatchMaxCount           int         `json:"batch_max_count,omitempty"`
	JitterBudgetMillis      int         `json:"jitter_budget_millis,omitempty"`
	DiscoveryLowInfo        bool        `json:"discovery_low_info,omitempty"`
}

type TargetIdentity struct {
	TargetID         string `json:"target_id"`
	Name             string `json:"name,omitempty"`
	LocalPath        string `json:"local_path,omitempty"`
	DevicePublicKey  string `json:"device_public_key,omitempty"`
	PairingReceiptID string `json:"pairing_receipt_id,omitempty"`
	PairedAt         string `json:"paired_at,omitempty"`
}

type NetworkConfig struct {
	ReceiverURL      string         `json:"receiver_url,omitempty"`
	LocalTLSIdentity TLSIdentityRef `json:"local_tls_identity,omitempty"`
}

type TLSIdentityRef struct {
	CertificatePath string `json:"certificate_path,omitempty"`
	PrivateKeyPath  string `json:"private_key_path,omitempty"`
}

type AgentKnowledge struct {
	Categories []KnowledgeCategory `json:"categories,omitempty"`
}

type KnowledgeCategory struct {
	Name     string   `json:"name"`
	Paths    []string `json:"paths,omitempty"`
	Manifest bool     `json:"manifest"`
}

func (p Profile) Validate() error {
	return p.validateWithOptions(profileValidationOptions{})
}

type profileValidationOptions struct {
	allowTargetIDLocalPathEquality bool
}

func (p Profile) validateWithOptions(opts profileValidationOptions) error {
	var errs []error

	if p.Version != CurrentVersion {
		errs = append(errs, fmt.Errorf("version must be %d", CurrentVersion))
	}
	if strings.TrimSpace(p.ProfileID) == "" {
		errs = append(errs, errors.New("profile_id is required"))
	}
	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if len(p.Roots) == 0 {
		errs = append(errs, errors.New("roots must contain at least one root"))
	}
	for i, root := range p.Roots {
		if strings.TrimSpace(root.ID) == "" {
			errs = append(errs, fmt.Errorf("roots[%d].id is required", i))
		}
		if strings.TrimSpace(root.Path) == "" {
			errs = append(errs, fmt.Errorf("roots[%d].path is required", i))
		}
	}
	for i, rule := range p.Include {
		if strings.TrimSpace(rule.Pattern) == "" {
			errs = append(errs, fmt.Errorf("include[%d].pattern is required", i))
		}
	}
	for i, rule := range p.Exclude {
		if strings.TrimSpace(rule.Pattern) == "" {
			errs = append(errs, fmt.Errorf("exclude[%d].pattern is required", i))
		}
	}
	if !validConsistency(p.Consistency) {
		errs = append(errs, fmt.Errorf("consistency must be one of live, strict, snapshot"))
	}
	if !validDeleteMode(p.DeletePolicy.Mode) {
		errs = append(errs, fmt.Errorf("delete_policy.mode must be one of ignore, record, prune"))
	}
	if p.DeletePolicy.Mode == DeleteModePrune && !p.DeletePolicy.RequireReview {
		errs = append(errs, errors.New("delete_policy.require_review must be true when mode is prune"))
	}
	if p.DeletePolicy.Mode == DeleteModePrune && !p.DeletePolicy.AllowPhysicalPrune {
		errs = append(errs, errors.New("delete_policy.allow_physical_prune must be true when mode is prune"))
	}
	if p.DeletePolicy.AllowPhysicalPrune && p.DeletePolicy.Mode != DeleteModePrune {
		errs = append(errs, errors.New("delete_policy.allow_physical_prune requires delete_policy.mode prune"))
	}
	if p.DeletePolicy.RetentionDays < 0 {
		errs = append(errs, errors.New("delete_policy.retention_days cannot be negative"))
	}
	if !validMetadataMode(p.MetadataPolicy.Mode) {
		errs = append(errs, fmt.Errorf("metadata_policy.mode must be one of basic, preserve"))
	}
	if !validPrivacyMode(p.PrivacyPolicy.Mode) {
		errs = append(errs, fmt.Errorf("privacy_policy.mode must be one of plaintext, redacted"))
	}
	if p.PrivacyPolicy.Mode == PrivacyModePlaintext && !p.PrivacyPolicy.AllowPlaintextRestore {
		errs = append(errs, errors.New("privacy_policy.allow_plaintext_restore must be true for plaintext mode"))
	}
	if p.PrivacyPolicy.TrafficLevel < 1 || p.PrivacyPolicy.TrafficLevel > 3 {
		errs = append(errs, errors.New("privacy_policy.traffic_level must be 1, 2, or 3"))
	}
	for name, value := range map[string]int{
		"padding_bucket_bytes": p.PrivacyPolicy.PaddingBucketBytes,
		"batch_max_bytes":      p.PrivacyPolicy.BatchMaxBytes,
		"batch_max_count":      p.PrivacyPolicy.BatchMaxCount,
		"jitter_budget_millis": p.PrivacyPolicy.JitterBudgetMillis,
	} {
		if value < 0 {
			errs = append(errs, fmt.Errorf("privacy_policy.%s cannot be negative", name))
		}
	}
	if p.PrivacyPolicy.TrafficLevel == 2 {
		if p.PrivacyPolicy.PaddingBucketBytes == 0 {
			errs = append(errs, errors.New("privacy_policy.padding_bucket_bytes is required for traffic level 2"))
		}
		if p.PrivacyPolicy.BatchMaxBytes == 0 || p.PrivacyPolicy.BatchMaxCount == 0 {
			errs = append(errs, errors.New("privacy_policy batching is required for traffic level 2"))
		}
		if p.PrivacyPolicy.JitterBudgetMillis == 0 {
			errs = append(errs, errors.New("privacy_policy.jitter_budget_millis is required for traffic level 2"))
		}
		if !p.PrivacyPolicy.DiscoveryLowInfo {
			errs = append(errs, errors.New("privacy_policy.discovery_low_info must be true for traffic level 2"))
		}
	}
	if strings.TrimSpace(p.Target.TargetID) == "" {
		errs = append(errs, errors.New("target.target_id is required"))
	}
	if strings.TrimSpace(p.Target.DevicePublicKey) != "" {
		if err := transport.DeviceID(p.Target.DevicePublicKey).Validate(); err != nil {
			errs = append(errs, fmt.Errorf("target.device_public_key is invalid: %w", err))
		}
	}
	if strings.TrimSpace(p.Target.PairingReceiptID) != "" {
		if err := control.ValidateArtifactID(p.Target.PairingReceiptID); err != nil {
			errs = append(errs, fmt.Errorf("target.pairing_receipt_id is unsafe: %w", err))
		}
	}
	if strings.TrimSpace(p.Target.PairedAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, p.Target.PairedAt); err != nil {
			errs = append(errs, fmt.Errorf("target.paired_at must be RFC3339 timestamp: %w", err))
		}
	}
	if strings.TrimSpace(p.Target.DevicePublicKey) == "" && (strings.TrimSpace(p.Target.PairingReceiptID) != "" || strings.TrimSpace(p.Target.PairedAt) != "") {
		errs = append(errs, errors.New("target.device_public_key is required when pairing_receipt_id or paired_at is set"))
	}
	if strings.TrimSpace(p.Target.PairingReceiptID) == "" && (strings.TrimSpace(p.Target.DevicePublicKey) != "" || strings.TrimSpace(p.Target.PairedAt) != "") {
		errs = append(errs, errors.New("target.pairing_receipt_id is required when device_public_key or paired_at is set"))
	}
	if strings.TrimSpace(p.Target.PairedAt) == "" && (strings.TrimSpace(p.Target.DevicePublicKey) != "" || strings.TrimSpace(p.Target.PairingReceiptID) != "") {
		errs = append(errs, errors.New("target.paired_at is required when device_public_key or pairing_receipt_id is set"))
	}
	if strings.TrimSpace(p.Target.LocalPath) != "" {
		cleanLocalPath := filepath.Clean(p.Target.LocalPath)
		if filepath.Clean(p.Target.TargetID) == cleanLocalPath && !opts.allowTargetIDLocalPathEquality {
			errs = append(errs, errors.New("target.target_id must not equal target.local_path"))
		}
	}
	if err := p.networkConfig().Validate(); err != nil {
		errs = append(errs, err)
	}
	for i, category := range p.AgentKnowledge.Categories {
		if strings.TrimSpace(category.Name) == "" {
			errs = append(errs, fmt.Errorf("agent_knowledge.categories[%d].name is required", i))
		}
	}

	return errors.Join(errs...)
}

func (p Profile) ValidateNetworkClientMaterial() error {
	if err := p.Validate(); err != nil {
		return err
	}
	network := p.networkConfig()
	if strings.TrimSpace(network.ReceiverURL) == "" {
		return errors.New("network.receiver_url is required for network client transfer")
	}
	if !network.LocalTLSIdentity.Configured() {
		return errors.New("network.local_tls_identity is required for network client transfer")
	}
	return nil
}

func (p Profile) ValidateNetworkServerMaterial() error {
	if err := p.Validate(); err != nil {
		return err
	}
	network := p.networkConfig()
	if strings.TrimSpace(network.ReceiverURL) == "" {
		return errors.New("network.receiver_url is required for network receiver serve")
	}
	if !network.LocalTLSIdentity.Configured() {
		return errors.New("network.local_tls_identity is required for network receiver serve")
	}
	return nil
}

func (p Profile) networkConfig() NetworkConfig {
	if p.Network == nil {
		return NetworkConfig{}
	}
	return *p.Network
}

func (n NetworkConfig) Validate() error {
	var errs []error
	if strings.TrimSpace(n.ReceiverURL) != "" {
		if err := validateReceiverURL(n.ReceiverURL); err != nil {
			errs = append(errs, fmt.Errorf("network.receiver_url is invalid: %w", err))
		}
	}
	if err := n.LocalTLSIdentity.Validate("network.local_tls_identity"); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (r TLSIdentityRef) Configured() bool {
	return strings.TrimSpace(r.CertificatePath) != "" && strings.TrimSpace(r.PrivateKeyPath) != ""
}

func (r TLSIdentityRef) Validate(prefix string) error {
	var errs []error
	cert := strings.TrimSpace(r.CertificatePath)
	key := strings.TrimSpace(r.PrivateKeyPath)
	if cert == "" && key == "" {
		return nil
	}
	if cert == "" {
		errs = append(errs, fmt.Errorf("%s.certificate_path is required when private_key_path is set", prefix))
	}
	if key == "" {
		errs = append(errs, fmt.Errorf("%s.private_key_path is required when certificate_path is set", prefix))
	}
	if cert != "" {
		if err := validateLocalIdentityPath(r.CertificatePath); err != nil {
			errs = append(errs, fmt.Errorf("%s.certificate_path is invalid: %w", prefix, err))
		}
	}
	if key != "" {
		if err := validateLocalIdentityPath(r.PrivateKeyPath); err != nil {
			errs = append(errs, fmt.Errorf("%s.private_key_path is invalid: %w", prefix, err))
		}
	}
	return errors.Join(errs...)
}

func validateReceiverURL(raw string) error {
	if strings.TrimSpace(raw) != raw {
		return errors.New("must not contain leading or trailing whitespace")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return errors.New("scheme must be https")
	}
	if parsed.User != nil {
		return errors.New("userinfo is not allowed")
	}
	if parsed.Host == "" {
		return errors.New("host is required")
	}
	if parsed.Port() == "" {
		return errors.New("explicit port is required")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if net.ParseIP(parsed.Hostname()) == nil {
		return errors.New("host must be an IP address")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return errors.New("path must be empty or /")
	}
	if parsed.RawQuery != "" {
		return errors.New("query is not allowed")
	}
	if parsed.Fragment != "" {
		return errors.New("fragment is not allowed")
	}
	return nil
}

func validateLocalIdentityPath(path string) error {
	if strings.TrimSpace(path) != path {
		return errors.New("must not contain leading or trailing whitespace")
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("path is required")
	}
	if strings.ContainsRune(path, '\x00') {
		return errors.New("must not contain NUL")
	}
	if strings.Contains(path, `\`) {
		return errors.New("must not contain backslash path separators")
	}
	if !filepath.IsAbs(path) {
		return errors.New("must be absolute")
	}
	if strings.HasSuffix(filepath.ToSlash(path), "/") {
		return errors.New("must name a file")
	}
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if segment == ".." {
			return errors.New("must not contain parent traversal")
		}
	}
	clean := filepath.Clean(path)
	if clean == "." || filepath.Dir(clean) == clean {
		return errors.New("path must name a file")
	}
	for _, segment := range strings.Split(filepath.ToSlash(clean), "/") {
		switch {
		case strings.EqualFold(segment, control.DirName):
			return errors.New("must not be stored under reserved .supermover control space")
		}
	}
	return nil
}

func ReadFile(path string) (Profile, error) {
	file, err := os.Open(path)
	if err != nil {
		return Profile{}, err
	}
	defer file.Close()
	return Read(file)
}

func Read(r io.Reader) (Profile, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	var p Profile
	if err := decoder.Decode(&p); err != nil {
		return Profile{}, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Profile{}, err
	}
	if err := p.Validate(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func ReadFileForTargetRepair(path string) (Profile, error) {
	file, err := os.Open(path)
	if err != nil {
		return Profile{}, err
	}
	defer file.Close()
	return ReadForTargetRepair(file)
}

func ReadForTargetRepair(r io.Reader) (Profile, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	var p Profile
	if err := decoder.Decode(&p); err != nil {
		return Profile{}, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Profile{}, err
	}
	if err := p.validateWithOptions(profileValidationOptions{allowTargetIDLocalPathEquality: true}); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON document")
		}
		return err
	}
	return nil
}

func WriteFile(path string, p Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".profile-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := Write(temp, p); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	return durable.SyncDirBestEffort(filepath.Dir(path))
}

func Write(w io.Writer, p Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(p)
}

func validConsistency(mode ConsistencyMode) bool {
	switch mode {
	case ConsistencyLive, ConsistencyStrict, ConsistencySnapshot:
		return true
	default:
		return false
	}
}

func validDeleteMode(mode DeleteMode) bool {
	switch mode {
	case DeleteModeIgnore, DeleteModeRecord, DeleteModePrune:
		return true
	default:
		return false
	}
}

func validMetadataMode(mode MetadataMode) bool {
	switch mode {
	case MetadataModeBasic, MetadataModePreserve:
		return true
	default:
		return false
	}
}

func validPrivacyMode(mode PrivacyMode) bool {
	switch mode {
	case PrivacyModePlaintext, PrivacyModeRedacted:
		return true
	default:
		return false
	}
}
