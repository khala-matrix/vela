package cmd

import (
	"fmt"

	"github.com/mars/vela/pkg/project"
	"github.com/mars/vela/pkg/state"
	"github.com/spf13/cobra"
)

var credentialsCmd = &cobra.Command{
	Use:   "credentials",
	Short: "Show database credentials for the current project",
	RunE:  runCredentials,
}

func runCredentials(cmd *cobra.Command, args []string) error {
	projectDir, err := project.Find(".")
	if err != nil {
		return err
	}

	b := &state.LocalBackend{}
	s, err := b.Load(projectDir)
	if err != nil {
		return err
	}

	if len(s.Credentials) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No credentials configured.")
		return nil
	}

	for name, cred := range s.Credentials {
		fmt.Fprintf(cmd.OutOrStdout(), "%s:\n", capitalize(name))
		fmt.Fprintf(cmd.OutOrStdout(), "  Host:     %s:%d\n", cred.Host, cred.Port)
		fmt.Fprintf(cmd.OutOrStdout(), "  Database: %s\n", cred.Database)
		fmt.Fprintf(cmd.OutOrStdout(), "  User:     %s\n", cred.User)
		fmt.Fprintf(cmd.OutOrStdout(), "  Password: %s\n", cred.Password)
		fmt.Fprintf(cmd.OutOrStdout(), "  URL:      postgresql://%s:%s@%s:%d/%s\n",
			cred.User, cred.Password, cred.Host, cred.Port, cred.Database)
	}
	return nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
