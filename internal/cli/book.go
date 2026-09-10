package cli

import (
	"github.com/amansk/spothero-pp-cli/internal/client"
	"github.com/amansk/spothero-pp-cli/internal/exitcode"
	"github.com/spf13/cobra"
)

const bookConfirmPhrase = "PLACE SPOTHERO BOOKING"

func newBookCmd(opt *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "book",
		Short: "Preview or place a parking booking (place is hard-gated)",
	}
	cmd.AddCommand(newBookPreviewCmd(opt))
	cmd.AddCommand(newBookPlaceCmd(opt))
	return cmd
}

func newBookPreviewCmd(opt *Options) *cobra.Command {
	var facilityID int
	var starts, ends, rateID, email string
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Preview a booking quote without charging",
		RunE: func(cmd *cobra.Command, args []string) error {
			if facilityID == 0 || starts == "" || ends == "" {
				return exitcode.Usagef("--facility-id --starts --ends are required")
			}
			c, err := opt.newClient()
			if err != nil {
				return err
			}
			rates, err := c.GetFacilityRates(facilityID, starts, ends)
			if err != nil {
				return err
			}
			preview := client.BookPreview{
				FacilityID: facilityID,
				Starts:     starts,
				Ends:       ends,
				DryRun:     true,
				Message:    "Preview only — no charge. Use book place with all safety gates to commit.",
			}
			if email == "" {
				if user, uerr := c.GetUser(); uerr == nil {
					preview.Email = user.Email
				}
			} else {
				preview.Email = email
			}
			if len(rates) > 0 {
				chosen := rates[0]
				if rateID != "" {
					for _, r := range rates {
						if r.RateID == rateID {
							chosen = r
							break
						}
					}
				}
				preview.RateID = chosen.RateID
				preview.PriceCents = chosen.PriceCents
				preview.Price = chosen.Price
			}
			return writeOut(cmd, opt, preview)
		},
	}
	cmd.Flags().IntVar(&facilityID, "facility-id", 0, "Facility ID from search results")
	cmd.Flags().StringVar(&starts, "starts", "", "Parking start datetime")
	cmd.Flags().StringVar(&ends, "ends", "", "Parking end datetime")
	cmd.Flags().StringVar(&rateID, "rate-id", "", "Optional specific rate ID")
	cmd.Flags().StringVar(&email, "email", "", "Receipt email override")
	return cmd
}

func newBookPlaceCmd(opt *Options) *cobra.Command {
	var facilityID int
	var starts, ends, rateID, email string
	var enableLive, ownerApproved bool
	var confirm string
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place a live SpotHero booking (requires explicit safety gates)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if facilityID == 0 || starts == "" || ends == "" {
				return exitcode.Usagef("--facility-id --starts --ends are required")
			}
			if !enableLive || !ownerApproved || confirm != bookConfirmPhrase {
				return exitcode.Usagef("refusing live booking: require --enable-live-booking --owner-approved --confirm %q", bookConfirmPhrase)
			}
			c, err := opt.newClient()
			if err != nil {
				return err
			}
			if c.Session == nil || c.Session.CookieHeader() == "" {
				return exitcode.Authf("authenticated session required; run auth login")
			}
			if rateID == "" {
				rates, err := c.GetFacilityRates(facilityID, starts, ends)
				if err != nil {
					return err
				}
				if len(rates) == 0 {
					return exitcode.NotFoundf("no rates for facility %d", facilityID)
				}
				rateID = rates[0].RateID
			}
			if email == "" {
				user, err := c.GetUser()
				if err != nil {
					return exitcode.Usagef("--email required when user profile unavailable: %v", err)
				}
				email = user.Email
			}
			req := client.CheckoutRequest{
				Items: []client.CheckoutItem{{
					FacilityID: facilityID,
					Starts:     starts,
					Ends:       ends,
					RateID:     rateID,
				}},
				Payment:  client.CheckoutPayment{},
				Currency: "USD",
				Email:    email,
			}
			if opt.DryRun {
				return writeOut(cmd, opt, map[string]any{
					"dry_run": true,
					"would_post": client.PathCheckout,
					"request": req,
				})
			}
			out, err := c.Checkout(req)
			if err != nil {
				return err
			}
			return writeOut(cmd, opt, map[string]any{"booked": true, "result": out})
		},
	}
	cmd.Flags().IntVar(&facilityID, "facility-id", 0, "Facility ID from search results")
	cmd.Flags().StringVar(&starts, "starts", "", "Parking start datetime")
	cmd.Flags().StringVar(&ends, "ends", "", "Parking end datetime")
	cmd.Flags().StringVar(&rateID, "rate-id", "", "Rate ID from book preview")
	cmd.Flags().StringVar(&email, "email", "", "Receipt email")
	cmd.Flags().BoolVar(&enableLive, "enable-live-booking", false, "Explicit opt-in to charge a payment method")
	cmd.Flags().BoolVar(&ownerApproved, "owner-approved", false, "Explicit owner approval for this booking")
	cmd.Flags().StringVar(&confirm, "confirm", "", "Must be exactly: "+bookConfirmPhrase)
	return cmd
}
