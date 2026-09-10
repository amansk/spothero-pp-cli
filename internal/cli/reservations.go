package cli

import (
	"github.com/amansk/spothero-pp-cli/internal/exitcode"
	"github.com/amansk/spothero-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newReservationsCmd(opt *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reservations",
		Aliases: []string{"reservation"},
		Short:   "List or fetch your SpotHero reservations",
	}
	cmd.AddCommand(newReservationsListCmd(opt))
	cmd.AddCommand(newReservationsGetCmd(opt))
	return cmd
}

func newReservationsListCmd(opt *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List reservations for the authenticated account",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := opt.newClient()
			if err != nil {
				return err
			}
			list, err := c.ListReservations()
			if err != nil {
				return err
			}
			if opt.JSON || opt.Agent {
				return writeOut(cmd, opt, map[string]any{"reservations": list, "count": len(list)})
			}
			if len(list) == 0 {
				_, _ = cmd.OutOrStdout().Write([]byte("No reservations.\n"))
				return nil
			}
			rows := make([][]string, 0, len(list))
			for _, r := range list {
				rows = append(rows, []string{r.ID, r.Status, r.FacilityTitle, r.Starts, r.Price})
			}
			return output.Table(cmd.OutOrStdout(), []string{"ID", "STATUS", "FACILITY", "STARTS", "PRICE"}, rows)
		},
	}
}

func newReservationsGetCmd(opt *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "get <reservation-id>",
		Short: "Get one reservation by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := opt.newClient()
			if err != nil {
				return err
			}
			res, err := c.GetReservation(args[0])
			if err != nil {
				return err
			}
			if res.ID == "" && res.RentalID == 0 {
				return exitcode.NotFoundf("reservation %s not found", args[0])
			}
			return writeOut(cmd, opt, res)
		},
	}
}
