// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"openbdap-pp-cli/internal/cliutil"
)

// newScaricaCmd scarica il CSV integrale di un dataset.
//
// Il comando emesso dal generatore per questo endpoint non funziona: pretende
// una risposta JSON e sul CSV esce con "API returned a non-JSON response".
// Qui la risposta si scrive cosi' com'e', a flusso, perche' i dump arrivano
// anche a centinaia di megabyte.
func newScaricaCmd(flags *rootFlags) *cobra.Command {
	var destinazione string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "scarica [dataset]",
		Short: "Scarica l'intero dataset in CSV (separatore punto e virgola)",
		Long: "Scarica il dump CSV completo di un dataset. Senza --output il contenuto va sullo standard output.\n" +
			"Usa questo comando per prendere tutto il dataset. NON usarlo per estrarre poche righe filtrate; usa 'righe'.",
		Example: strings.Trim(`
  openbdap-pp-cli scarica d032b3a2-2b70-4193-a0c8-cb7eb69f8710 --output conto-economico.csv
  openbdap-pp-cli scarica d032b3a2-2b70-4193-a0c8-cb7eb69f8710 | head -5
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "dataset=d032b3a2-2b70-4193-a0c8-cb7eb69f8710",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "scarica")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("indica il dataset da scaricare"))
			}
			id := args[0]
			if d, ok, err := risolviLocaleMorbido(cmd, flags, dbPath, args[0]); err != nil {
				return err
			} else if ok && d.ID != "" {
				id = d.ID
			}
			// L'indirizzo http pubblicato nei metadati non risponde: serve https.
			url := baseSito + pathDumpPrefix + id + ".csv"

			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Accept", "text/csv")
			client := &http.Client{Timeout: 0, Transport: http.DefaultTransport}
			if cliutil.IsDogfoodEnv() {
				client.Timeout = 20 * time.Second
			}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("scaricamento di %s: %w", id, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("il portale ha risposto %s per il dataset %s", resp.Status, id)
			}

			destinatario := cmd.OutOrStdout()
			var file *os.File
			if destinazione != "" {
				file, err = os.Create(destinazione)
				if err != nil {
					return err
				}
				defer file.Close()
				destinatario = file
			}
			scritti, err := io.Copy(destinatario, resp.Body)
			if err != nil {
				return fmt.Errorf("scrittura del CSV: %w", err)
			}
			if destinazione != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "scritti %d byte in %s\n", scritti, destinazione)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&destinazione, "output", "", "scrivi il CSV in un file invece che sullo standard output")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}
