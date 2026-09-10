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
	var starts, ends, rateID, email, citySlug string
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Preview a booking quote without charging",
		Long:  "Fetches quote via GET api.spothero.com/v2/search/transient/{facilityId} (no charge). Naive --starts/--ends use --city-slug for local timezone (same as search); pass RFC3339 with Z/offset to override.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if facilityID == 0 || starts == "" || ends == "" {
				return exitcode.Usagef("--facility-id --starts --ends are required")
			}
			c, err := opt.newClient()
			if err != nil {
				return err
			}
			quote, err := c.GetFacilityRates(client.FacilityRateQuery{
				FacilityID: facilityID,
				Starts:     starts,
				Ends:       ends,
				CitySlug:   citySlug,
			})
			if err != nil {
				return err
			}
			preview := client.BookPreview{
				FacilityID:   facilityID,
				Title:        quote.Title,
				Starts:       starts,
				Ends:         ends,
				PeriodsUTC:   quote.PeriodsUTC,
				TimezoneNote: quote.TimezoneNote,
				DryRun:       true,
				Message:      "Preview only — no charge. Use book place with all safety gates to commit.",
			}
			preview.Email, _ = c.ResolveEmail(email)
			if len(quote.Rates) > 0 {
				chosen := quote.Rates[0]
				if rateID != "" {
					for _, r := range quote.Rates {
						if r.RateID == rateID || r.QuoteToken == rateID {
							chosen = r
							break
						}
					}
				}
				client.ApplyBookPreviewFromRate(&preview, chosen)
			}
			return writeOut(cmd, opt, preview)
		},
	}
	cmd.Flags().IntVar(&facilityID, "facility-id", 0, "Facility ID from search results")
	cmd.Flags().StringVar(&starts, "starts", "", "Parking start datetime")
	cmd.Flags().StringVar(&ends, "ends", "", "Parking end datetime")
	cmd.Flags().StringVar(&citySlug, "city-slug", "", "Search city slug for naive datetime TZ (from search params.city_slug)")
	cmd.Flags().StringVar(&rateID, "rate-id", "", "Optional specific rate id or quote token")
	cmd.Flags().StringVar(&email, "email", "", "Receipt email override")
	return cmd
}

func newBookPlaceCmd(opt *Options) *cobra.Command {
	var facilityID int
	var starts, ends, rateID, email, citySlug string
	var vehicleProfileID, cardID int
	var licensePlate, licensePlateState string
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
			req, _, err := c.BuildCheckout(client.BookPlaceInput{
				FacilityID:        facilityID,
				Starts:            starts,
				Ends:              ends,
				CitySlug:          citySlug,
				Email:             email,
				SelectRateID:      rateID,
				VehicleProfileID:  vehicleProfileID,
				CardID:            cardID,
				LicensePlateStr:   licensePlate,
				LicensePlateState: licensePlateState,
			})
			if err != nil {
				return err
			}
			if opt.DryRun {
				return writeOut(cmd, opt, map[string]any{
					"dry_run":    true,
					"would_post": client.PathCheckout,
					"request":    req,
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
	cmd.Flags().StringVar(&citySlug, "city-slug", "", "Search city slug for naive datetime TZ")
	cmd.Flags().StringVar(&rateID, "rate-id", "", "Rate id or quote token from book preview")
	cmd.Flags().StringVar(&email, "email", "", "Receipt email")
	cmd.Flags().IntVar(&vehicleProfileID, "vehicle-profile-id", 0, "Saved vehicle profile id for item_context")
	cmd.Flags().IntVar(&cardID, "card-id", 0, "Saved payment card id")
	cmd.Flags().StringVar(&licensePlate, "license-plate", "", "Ad-hoc license plate when required")
	cmd.Flags().StringVar(&licensePlateState, "license-plate-state", "", "Ad-hoc license plate state when required")
	cmd.Flags().BoolVar(&enableLive, "enable-live-booking", false, "Explicit opt-in to charge a payment method")
	cmd.Flags().BoolVar(&ownerApproved, "owner-approved", false, "Explicit owner approval for this booking")
	cmd.Flags().StringVar(&confirm, "confirm", "", "Must be exactly: "+bookConfirmPhrase)
	return cmd
}
