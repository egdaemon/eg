package compute

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/egdaemon/eg/compute"
)

type pubCmd struct {
	List   pubList   `cmd:"" name:"ls" help:"list the runners of the account"`
	Remove pubRemove `cmd:"" name:"rm" help:"remove runners from the account, a runner that is still running registers itself again"`
}

// render the runners as a table. runners shared with other accounts are marked in the table as well as styled
// so they remain visible without color.
func render(runners ...*compute.Compute) string {
	var (
		bold   = lipgloss.NewStyle().Bold(true)
		marked = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
		rows   = make([][]string, 0, len(runners))
		shared = make(map[int]bool, len(runners))
	)

	for i, r := range runners {
		mark := ""
		if r.GetGlobal() {
			mark = "*"
			shared[i] = true
		}

		rows = append(rows, []string{
			mark,
			fmt.Sprintf("%v", TypographyRunnerCompact(r)),
			fmt.Sprintf("%v", TypographyRunnerPlatform(r)),
			fmt.Sprintf("%d", r.GetCores()),
			fmt.Sprintf("%v", TypographyBytes(r.GetMemory())),
			fmt.Sprintf("%v", TypographyBytes(r.GetVram())),
			fmt.Sprintf("%v", TypographyRunnerLabels(r)),
			fmt.Sprintf("%v", TypographyRunnerLastSeen(r)),
		})
	}

	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("", "RUNNER", "OS/ARCH", "CORES", "MEMORY", "VRAM", "LABELS", "LAST SEEN").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return bold.Padding(0, 1)
			case shared[row]:
				return marked.Padding(0, 1)
			default:
				return lipgloss.NewStyle().Padding(0, 1)
			}
		})

	return fmt.Sprintf("runners: %d, * shared with other accounts\n\n%s", len(runners), tbl.String())
}
