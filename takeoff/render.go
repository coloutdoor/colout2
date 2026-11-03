package takeoff

import (
	"fmt"
	"io"
)

// Render writes the take off as Markdown in the standard template.
func Render(w io.Writer, estimateID string, res Result) error {
	d := res.Deck
	if _, err := fmt.Fprintf(w, "# Materials Take Off - Estimate %s\n%g' projection x %g' wide, %g' high, %s\n",
		estimateID, d.ProjectionFt, d.WidthFt, d.HeightFt, d.Material); err != nil {
		return err
	}
	if res.Stop != "" {
		_, err := fmt.Fprintf(w, "\n%s\n", res.Stop)
		return err
	}
	for _, list := range res.Lists {
		if _, err := fmt.Fprintf(w, "\n## %s\n\n| COUNT | NAME | DESCRIPTION | NOTES |\n|-------|------|-------------|-------|\n", list.Title); err != nil {
			return err
		}
		for _, item := range list.Items {
			if _, err := fmt.Fprintf(w, "| %d | %s | %s | %s |\n", item.Count, item.Name, item.Description, item.Notes); err != nil {
				return err
			}
		}
	}
	return nil
}
