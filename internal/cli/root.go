// Package cli is the cobra command tree for spothero-pp-cli.
package cli

import (
	"fmt"
	"os"

	"github.com/amansk/spothero-pp-cli/internal/auth"
	"github.com/amansk/spothero-pp-cli/internal/client"
	"github.com/amansk/spothero-pp-cli/internal/exitcode"
	"github.com/amansk/spothero-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

var version = "0.1.7"

// Options are global CLI flags.
type Options struct {
	JSON    bool
	Agent   bool
	Quiet   bool
	NoColor bool
	NoInput bool
	Yes     bool
	DryRun  bool
	Home    string
	HTTP    *client.Client
}

func (o Options) Mode() output.Mode {
	return output.Mode{JSON: o.JSON || o.Agent, Agent: o.Agent, Quiet: o.Quiet, NoColor: o.NoColor || o.Agent}
}

func (o *Options) ResolveHome() (string, error) {
	return auth.HomeDir(o.Home)
}

func (o *Options) newClient() (*client.Client, error) {
	if o.HTTP != nil {
		o.HTTP.DryRun = o.DryRun
		return o.HTTP, nil
	}
	home, err := o.ResolveHome()
	if err != nil {
		return nil, err
	}
	sess, err := auth.ResolveSession(home)
	if err != nil {
		return nil, err
	}
	c := client.New(sess)
	c.DryRun = o.DryRun
	return c, nil
}

func newRoot(opt *Options) *cobra.Command {
	if opt == nil {
		opt = &Options{}
	}
	cmd := &cobra.Command{
		Use:           "spothero-pp-cli",
		Short:         "Agent-native SpotHero consumer parking CLI",
		Long:          "Search, list, and (hard-gated) book or cancel SpotHero consumer reservations via spothero.com session cookies. Never prints secrets.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if opt.Agent {
				opt.JSON = true
				opt.NoColor = true
				opt.NoInput = true
			}
		},
	}
	cmd.PersistentFlags().BoolVar(&opt.JSON, "json", false, "Emit machine-readable JSON")
	cmd.PersistentFlags().BoolVar(&opt.Agent, "agent", false, "Agent mode: --json --no-color --no-input (does not imply --yes)")
	cmd.PersistentFlags().BoolVar(&opt.Quiet, "quiet", false, "Suppress non-essential output")
	cmd.PersistentFlags().BoolVar(&opt.NoColor, "no-color", false, "Disable color")
	cmd.PersistentFlags().BoolVar(&opt.NoInput, "no-input", false, "Never prompt interactively")
	cmd.PersistentFlags().BoolVar(&opt.Yes, "yes", false, "Auto-confirm when a command allows it")
	cmd.PersistentFlags().BoolVar(&opt.DryRun, "dry-run", false, "Validate requests without mutating remote state")
	cmd.PersistentFlags().StringVar(&opt.Home, "home", "", "Override config dir ($SPOTHERO_PP_HOME or ~/.config/spothero-pp-cli)")

	cmd.AddCommand(newAuthCmd(opt))
	cmd.AddCommand(newAccountCmd(opt))
	cmd.AddCommand(newDoctorCmd(opt))
	cmd.AddCommand(newSearchCmd(opt))
	cmd.AddCommand(newReservationsCmd(opt))
	cmd.AddCommand(newBookCmd(opt))
	cmd.AddCommand(newCancelCmd(opt))
	return cmd
}

// Execute runs the CLI and returns a typed exit code.
func Execute() int {
	cmd := newRoot(nil)
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	if err := cmd.Execute(); err != nil {
		return handleErr(cmd, err)
	}
	return exitcode.OK
}

func handleErr(cmd *cobra.Command, err error) int {
	code := exitcode.Usage
	var ex *exitcode.Error
	if exitcode.As(err, &ex) {
		code = ex.Code
		if ex.Silent {
			return code
		}
	}
	jsonFlag, _ := cmd.Root().PersistentFlags().GetBool("json")
	agentFlag, _ := cmd.Root().PersistentFlags().GetBool("agent")
	mode := output.Mode{JSON: jsonFlag || agentFlag, Agent: agentFlag}
	if encErr := mode.EncodeError(cmd.ErrOrStderr(), err); encErr != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return code
}

func writeOut(cmd *cobra.Command, opt *Options, data any) error {
	return opt.Mode().Encode(cmd.OutOrStdout(), data)
}

// NewRootForTest exposes the command tree for unit tests.
func NewRootForTest(opt *Options) *cobra.Command {
	return newRoot(opt)
}

// ExitCodeForTest maps errors to exit codes in tests.
func ExitCodeForTest(err error) int {
	code := exitcode.Usage
	var ex *exitcode.Error
	if exitcode.As(err, &ex) {
		return ex.Code
	}
	return code
}
