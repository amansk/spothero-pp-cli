package cli

import (
	"github.com/amansk/spothero-pp-cli/internal/exitcode"
	"github.com/spf13/cobra"
)

func newAccountCmd(opt *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Read consumer account profile data (no secrets)",
	}
	cmd.AddCommand(newAccountCardsCmd(opt))
	return cmd
}

func newAccountCardsCmd(opt *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "cards",
		Short: "List saved payment cards (last4 and ids only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := opt.newClient()
			if err != nil {
				return err
			}
			if c.Session == nil || c.Session.CookieHeader() == "" {
				return exitcode.Authf("authenticated session required; run auth login")
			}
			me, err := c.GetMe()
			if err != nil {
				return err
			}
			me, err = c.EnrichCreditCards(me)
			if err != nil {
				return err
			}
			out := make([]map[string]any, 0, len(me.CreditCards))
			for _, card := range me.CreditCards {
				item := map[string]any{
					"card_id": card.CardID,
				}
				if card.CardExternalID != "" {
					item["card_external_id"] = card.CardExternalID
				}
				if card.CardLast4 != "" {
					item["card_last4"] = card.CardLast4
				}
				if card.IsDefault {
					item["is_default"] = true
				}
				out = append(out, item)
			}
			return writeOut(cmd, opt, map[string]any{
				"user_id": me.ID,
				"cards":   out,
				"count":   len(out),
			})
		},
	}
}
