package commands

import (
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

// profileEntry describes one AWS CLI profile: whether credentials exist for it
// (credentials file) or it is a role-chaining profile (config file), and when
// a SAML session expires if the provider recorded that.
func profileEntry(profile string, credentials, configSections []iniSection, now time.Time) map[string]any {
	entry := map[string]any{"profile": profile, "present": false, "source": "none"}
	if section, ok := findINISection(credentials, profile); ok {
		entry["present"] = true
		entry["source"] = "credentials"
		if expires, ok := sectionExpiry(section); ok {
			entry["expires_at"] = expires.UTC().Format(time.RFC3339)
			entry["expired"] = !expires.After(now)
			remaining := int(expires.Sub(now).Seconds())
			if remaining < 0 {
				remaining = 0
			}
			entry["seconds_remaining"] = remaining
		}
		if arn := section.Values["x_principal_arn"]; arn != "" {
			entry["principal_arn"] = arn
		}
		return entry
	}
	if section, ok := findINISection(configSections, awsConfigSectionName(profile)); ok && section.Values["role_arn"] != "" {
		entry["present"] = true
		entry["source"] = "config"
		entry["role_arn"] = section.Values["role_arn"]
		if source := section.Values["source_profile"]; source != "" {
			entry["source_profile"] = source
		}
	}
	return entry
}

func statusCmd(o *Opts) *cobra.Command {
	var account string
	var verify bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Report which AWS CLI profiles hold credentials and whether their session expired.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var aws config.AWSConfig
			if _, cfg, err := loadAWSConfigForRead(o.Config); err == nil {
				aws = cfg.AWS
			}
			credentialsPath := awsCredentialsFilePath()
			configPath := awsConfigFilePath()
			credentials, credentialsErr := readINI(credentialsPath)
			configSections, configErr := readINI(configPath)
			now := time.Now()

			var filter accountSelection
			if account != "" {
				sel, failure := resolveAccount(aws, account)
				if failure != nil {
					return print(cmd, o, *failure)
				}
				filter = sel
			}

			profiles := make([]map[string]any, 0)
			seen := map[string]bool{}
			for _, acct := range aws.EnabledAccounts() {
				if account != "" && !(filter.Configured && acct.EffectiveProfile() == filter.Account.EffectiveProfile()) {
					continue
				}
				profile := acct.EffectiveProfile()
				entry := profileEntry(profile, credentials, configSections, now)
				entry["account"] = acct.Name
				entry["account_id"] = acct.AccountID
				entry["role"] = acct.Role
				if len(acct.Regions) > 0 {
					entry["region"] = acct.Regions[0]
				} else if aws.DefaultRegion != "" {
					entry["region"] = aws.DefaultRegion
				}
				seen[profile] = true
				profiles = append(profiles, entry)
			}
			if account != "" && !filter.Configured && filter.Account.AccountID != "" {
				entry := profileEntry(defaultAWSAuthProfile, credentials, configSections, now)
				entry["account_id"] = filter.Account.AccountID
				seen[defaultAWSAuthProfile] = true
				profiles = append(profiles, entry)
			}
			if account == "" {
				for _, section := range credentials {
					if seen[section.Name] {
						continue
					}
					seen[section.Name] = true
					profiles = append(profiles, profileEntry(section.Name, credentials, configSections, now))
				}
			}

			presentCount, expiredCount := 0, 0
			for _, entry := range profiles {
				if entry["present"] == true {
					presentCount++
				}
				if entry["expired"] == true {
					expiredCount++
				}
				if !verify || entry["present"] != true {
					continue
				}
				region, _ := entry["region"].(string)
				profile, _ := entry["profile"].(string)
				identity, err := verifyIdentity(cmd.Context(), o.runner, profile, region)
				if err != nil {
					entry["verified"] = false
					entry["verify_error"] = redactWithSecrets(err.Error(), aws.Password)
					continue
				}
				entry["verified"] = true
				entry["identity"] = identity.view()
				if expected, _ := entry["account_id"].(string); expected != "" && identity.Account != expected {
					entry["verified"] = false
					entry["verify_error"] = "credentials belong to account " + identity.Account + ", expected " + expected
				}
			}

			data := map[string]any{
				"provider":         aws.EffectiveProvider(),
				"credentials_file": credentialsPath,
				"config_file":      configPath,
				"profiles":         profiles,
				"present_count":    presentCount,
				"expired_count":    expiredCount,
				"checked_at":       now.UTC().Format(time.RFC3339),
			}
			if credentialsErr != nil {
				data["credentials_file_error"] = output.RedactString(strings.TrimSpace(credentialsErr.Error()))
			}
			if configErr != nil {
				data["config_file_error"] = output.RedactString(strings.TrimSpace(configErr.Error()))
			}
			return print(cmd, o, output.Success("", data))
		},
	}
	c.Flags().StringVar(&account, "account", "", "Only report the profile of this configured account (name or id).")
	c.Flags().BoolVar(&verify, "verify", false, "Call aws sts get-caller-identity for every present profile.")
	return c
}
