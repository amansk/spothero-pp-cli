package cli

import (
	"github.com/amansk/spothero-pp-cli/internal/auth"
	"github.com/amansk/spothero-pp-cli/internal/client"
	"github.com/spf13/cobra"
)

func newDoctorCmd(opt *Options) *cobra.Command {
	var live bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local setup and optional live API connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := opt.ResolveHome()
			if err != nil {
				return err
			}
			sess, err := auth.ResolveSession(home)
			if err != nil {
				return err
			}
			report := map[string]any{
				"ok":          true,
				"cli_version": version,
				"home":        home,
				"api_base":    client.DefaultBaseURL,
				"auth_type":   "cookie",
				"auth":        sess.Status(),
				"checks":      []map[string]any{},
			}
			checks := report["checks"].([]map[string]any)
			checks = append(checks, map[string]any{"name": "config_dir", "ok": true, "detail": home})
			if sess == nil || sess.CookieHeader() == "" {
				checks = append(checks, map[string]any{"name": "session", "ok": false, "detail": "not authenticated; run auth login"})
				report["ok"] = false
			} else {
				checks = append(checks, map[string]any{"name": "session", "ok": true, "detail": "cookies present"})
			}
			if live {
				c, err := opt.newClient()
				if err != nil {
					return err
				}
				if err := c.Ping(); err != nil {
					checks = append(checks, map[string]any{"name": "live_ping", "ok": false, "detail": err.Error()})
					report["ok"] = false
				} else {
					checks = append(checks, map[string]any{"name": "live_ping", "ok": true, "detail": "search-params reachable"})
				}
				if sess != nil && sess.CookieHeader() != "" {
					if _, err := c.GetUser(); err != nil {
						checks = append(checks, map[string]any{"name": "live_user", "ok": false, "detail": err.Error()})
						report["ok"] = false
					} else {
						checks = append(checks, map[string]any{"name": "live_user", "ok": true, "detail": "session valid"})
					}
				}
			}
			report["checks"] = checks
			return writeOut(cmd, opt, report)
		},
	}
	cmd.Flags().BoolVar(&live, "live", false, "Probe live spothero.com consumer API (read-only)")
	return cmd
}
