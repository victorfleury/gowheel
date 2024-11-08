package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"net/http"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/net/html"
	dl "gowheel/internal/pkg/dl"
)

var PYPI_SIMPLE_URL = "https://pypi.org/simple/{PACKAGE}/"
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List versions of a given Python package.",
	Long:  `List all the versions available for a given Python package. This uses the data available on PyPI at https://pypi.org/simple/<package>`,
	//Run: func(cmd *cobra.Command, args []string) {
	//listPackages(cmd, args)
	//},
	Run: func(cmd *cobra.Command, args []string) {
		main(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

// List model bit
type model struct {
	list     list.Model
	cursor   int
	choice   string
	selected map[int]struct{}
	quitting bool
	path     string
}

type item string

var wheelsURL = make(map[string]string)

func (i item) FilterValue() string { return string(i) }

type itemDelegate struct{}

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

func listPackages(cmd *cobra.Command, args []string) []list.Item {
	cmd.Name()
	fmt.Println("Fetching PyPI wheels versions of " + args[0])

	url_replacement := []string{"{PACKAGE}", args[0]}
	url := strings.NewReplacer(url_replacement...).Replace(PYPI_SIMPLE_URL)
	fmt.Println("Fetching from " + url)

	client := &http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Could not build request", err)
	}
	response, err := client.Do(request)
	if err != nil {
		log.Fatalf("Response is empty", err)
	}

	defer response.Body.Close()

	if err != nil {
		log.Fatal("Oh no....", err)
	}

	document, err := html.Parse(response.Body)

	// Recursive parsing of the html document to find versions
	var findVersion func(*html.Node)
	var versions []list.Item
	findVersion = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			versions = append(versions, item(n.FirstChild.Data))
			for _, a := range n.Attr {
				if a.Key == "href" {
					wheelsURL[n.FirstChild.Data] = a.Val
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findVersion(c)
		}
	}
	findVersion(document)
	return versions
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = string(i)
			}
			path, err := dl.DownloadWheel(wheelsURL[m.choice])
			if err != nil {
				fmt.Println("Oh no")
			}
			m.path = path
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.choice != "" {
		//if err != nil {
		//quitTextStyle.Render("Oh no could not download wheel")
		//}
		return quitTextStyle.Render(fmt.Sprintf("Wheel downloaded at %s", m.path))

	}
	if m.quitting {
		return quitTextStyle.Render("Bye !")
	}
	return "\n" + m.list.View()
}

func main(cmd *cobra.Command, args []string) {
	versions := listPackages(cmd, args)

	l := list.New(versions, itemDelegate{}, 50, 34)
	l.Title = fmt.Sprintf("Which wheels for %s would you like to download ?", args[0])
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	m := model{list: l}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
