package cli

import (
	"github.com/amansk/spothero-pp-cli/internal/client"
	"github.com/amansk/spothero-pp-cli/internal/exitcode"
	"github.com/spf13/cobra"
)

func newCancelCmd(opt *Options) *cobra.Command {
	var yes bool
	var confirm string
	cmd := &cobra.Command{
		Use:   "cancel [reservation-id]",
		Short: "Preview or execute a reservation cancellation",
		Long:  "Run `cancel preview <id>` to obtain a confirm token, then `cancel <id> --yes --confirm <token>`.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return exitcode.Usagef("provide reservation-id or use: cancel preview <id>")
			}
			if !yes || confirm == "" {
				return exitcode.Usagef("refusing cancel: run `cancel preview %s` first, then pass --yes --confirm <token>", args[0])
			}
			home, err := opt.ResolveHome()
			if err != nil {
				return err
			}
			c, err := opt.newClient()
			if err != nil {
				return err
			}
			if opt.DryRun {
				if err := validateCancelToken(home, confirm, args[0]); err != nil {
					return exitcode.Usagef("%v", err)
				}
				return writeOut(cmd, opt, map[string]any{
					"dry_run":    true,
					"would_post": client.PathReservationRefund,
					"id":         args[0],
				})
			}
			if err := consumeCancelToken(home, confirm, args[0]); err != nil {
				return exitcode.Usagef("%v", err)
			}
			res, err := c.GetReservation(args[0])
			if err != nil {
				return err
			}
			if !res.Cancellable {
				return exitcode.Usagef("reservation %s is not refundable", args[0])
			}
			out, err := c.RefundReservation(args[0])
			if err != nil {
				return err
			}
			return writeOut(cmd, opt, map[string]any{"cancelled": true, "result": out})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm cancellation")
	cmd.Flags().StringVar(&confirm, "confirm", "", "Token from cancel preview")
	cmd.AddCommand(newCancelPreviewCmd(opt))
	return cmd
}

func newCancelPreviewCmd(opt *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "preview <reservation-id>",
		Short: "Preview cancellation and obtain a confirm token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := opt.ResolveHome()
			if err != nil {
				return err
			}
			c, err := opt.newClient()
			if err != nil {
				return err
			}
			res, err := c.GetReservation(args[0])
			if err != nil {
				return err
			}
			if !res.Cancellable {
				return exitcode.Usagef("reservation %s is not refundable; cancel not available", args[0])
			}
			token, err := issueCancelToken(home, args[0])
			if err != nil {
				return err
			}
			preview := client.CancelPreview{
				ReservationID: args[0],
				Status:        res.Status,
				Refundable:    res.Cancellable,
				ConfirmToken:  token,
				Message:       "Run: spothero-pp-cli cancel " + args[0] + " --yes --confirm <token>",
			}
			return writeOut(cmd, opt, preview)
		},
	}
}
