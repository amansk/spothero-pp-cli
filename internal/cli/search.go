package cli

import (
	"strconv"
	"strings"

	"github.com/amansk/spothero-pp-cli/internal/client"
	"github.com/amansk/spothero-pp-cli/internal/exitcode"
	"github.com/amansk/spothero-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newSearchCmd(opt *Options) *cobra.Command {
	var address string
	var lat, lng float64
	var starts, ends string
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search parking near an address or coordinates",
		Long:  "Geocodes via spothero.com search-params, then GET api.spothero.com/v2/search/transient (live HAR path). Naive --starts/--ends are interpreted in the search city's local timezone when known; use RFC3339 with Z or offset to override.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if starts == "" || ends == "" {
				return exitcode.Usagef("--starts and --ends are required (e.g. 2026-09-15T09:00)")
			}
			hasAddr := strings.TrimSpace(address) != ""
			hasCoords := lat != 0 || lng != 0
			if hasAddr == hasCoords {
				return exitcode.Usagef("provide exactly one of --address or --lat/--lng")
			}
			c, err := opt.newClient()
			if err != nil {
				return err
			}
			q := client.SearchQuery{Starts: starts, Ends: ends}
			if hasAddr {
				q.Address = address
			} else {
				q.Latitude = lat
				q.Longitude = lng
			}
			result, err := c.Search(q)
			if err != nil {
				return err
			}
			if opt.JSON || opt.Agent {
				return writeOut(cmd, opt, result)
			}
			if len(result.Results) == 0 {
				_, _ = cmd.OutOrStdout().Write([]byte("No parking spots found for this window.\n"))
				return nil
			}
			rows := make([][]string, 0, len(result.Results))
			for _, s := range result.Results {
				rows = append(rows, []string{
					strconv.Itoa(s.FacilityID),
					s.Title,
					s.Price,
					strconv.Itoa(s.DistanceMeters),
				})
			}
			return output.Table(cmd.OutOrStdout(), []string{"ID", "TITLE", "PRICE", "WALK_M"}, rows)
		},
	}
	cmd.Flags().StringVar(&address, "address", "", "Street address or POI to search near")
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude")
	cmd.Flags().Float64Var(&lng, "lng", 0, "Longitude")
	cmd.Flags().StringVar(&starts, "starts", "", "Parking start (local or RFC3339)")
	cmd.Flags().StringVar(&ends, "ends", "", "Parking end (local or RFC3339)")
	return cmd
}
