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
		RunE: func(cmd *cobra.Command, args []string) error {
			if starts == "" || ends == "" {
				return exitcode.Usagef("--starts and --ends are required (e.g. 2026-09-11T09:30)")
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
			if len(result.Facilities) == 0 {
				msg := "No facilities found via GET /api/v1/facilities/.\n"
				if result.EmptyHint != "" {
					msg += result.EmptyHint + "\n"
				}
				_, _ = cmd.OutOrStdout().Write([]byte(msg))
				return nil
			}
			rows := make([][]string, 0, len(result.Facilities))
			for _, f := range result.Facilities {
				rows = append(rows, []string{
					strconv.Itoa(f.ID),
					f.Title,
					f.Price,
					strconv.FormatFloat(f.Distance, 'f', 2, 64),
				})
			}
			return output.Table(cmd.OutOrStdout(), []string{"ID", "TITLE", "PRICE", "DIST_M"}, rows)
		},
	}
	cmd.Flags().StringVar(&address, "address", "", "Street address or POI to search near")
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude")
	cmd.Flags().Float64Var(&lng, "lng", 0, "Longitude")
	cmd.Flags().StringVar(&starts, "starts", "", "Parking start (local or ISO datetime)")
	cmd.Flags().StringVar(&ends, "ends", "", "Parking end (local or ISO datetime)")
	return cmd
}
