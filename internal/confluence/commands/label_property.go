package commands

import "github.com/spf13/cobra"

func labelCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "label"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		p, _ := cmd.Flags().GetString("prefix")
		q := map[string]string{}
		if p != "" {
			q["prefix"] = p
		}
		return do(o, cmd, "GET", "label", q, nil)
	}})
	c.Commands()[0].Flags().String("prefix", "", "")
	return c
}
