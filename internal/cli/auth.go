package cli

import (
	"os"

	"github.com/amansk/spothero-pp-cli/internal/auth"
	"github.com/amansk/spothero-pp-cli/internal/exitcode"
	"github.com/spf13/cobra"
)

func newAuthCmd(opt *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage SpotHero session authentication",
	}
	cmd.AddCommand(newAuthLoginCmd(opt))
	cmd.AddCommand(newAuthStatusCmd(opt))
	return cmd
}

func newAuthLoginCmd(opt *Options) *cobra.Command {
	var useChrome bool
	var cookiesFile string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Import spothero.com session cookies",
		Long:  "Sign in at spothero.com in Chrome, then run with --chrome, or export Playwright storage-state JSON / raw Cookie header via --cookies-file. Cookie files are stored mode 0600; secrets are never printed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if useChrome == (cookiesFile != "") {
				if !useChrome && cookiesFile == "" {
					return exitcode.Usagef("specify exactly one of --chrome or --cookies-file")
				}
				return exitcode.Usagef("use only one of --chrome or --cookies-file")
			}
			home, err := opt.ResolveHome()
			if err != nil {
				return err
			}
			var sess *auth.Session
			switch {
			case useChrome:
				sess, err = auth.ImportChromeCookies()
			case cookiesFile != "":
				data, err := os.ReadFile(cookiesFile)
				if err != nil {
					return exitcode.Usagef("read cookies file: %v", err)
				}
				sess, err = auth.ParseCookiesFile(data, "cookies-file:"+cookiesFile)
			}
			if err != nil {
				return exitcode.Authf("%v", err)
			}
			if err := auth.SaveSession(home, sess); err != nil {
				return err
			}
			status := sess.Status()
			status["message"] = "session saved"
			status["home"] = home
			return writeOut(cmd, opt, status)
		},
	}
	cmd.Flags().BoolVar(&useChrome, "chrome", false, "Import cookies from local Chrome (sign in at spothero.com first)")
	cmd.Flags().StringVar(&cookiesFile, "cookies-file", "", "Playwright storage-state JSON or raw Cookie header file")
	return cmd
}

func newAuthStatusCmd(opt *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status (no secrets)",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := opt.ResolveHome()
			if err != nil {
				return err
			}
			sess, err := auth.ResolveSession(home)
			if err != nil {
				return err
			}
			out := map[string]any{"home": home}
			if sess == nil {
				out["authenticated"] = false
				out["hint"] = "run: spothero-pp-cli auth login --chrome"
				return writeOut(cmd, opt, out)
			}
			for k, v := range sess.Status() {
				out[k] = v
			}
			if env := os.Getenv(auth.CookiesEnv); env != "" {
				out["env_override"] = auth.CookiesEnv
			}
			return writeOut(cmd, opt, out)
		},
	}
}
