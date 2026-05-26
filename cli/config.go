package cli

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// AddConfigCommands registers APImetrics-specific configuration commands on
// the root command. Called from cli.Init after Root is created.
func AddConfigCommands(root *cobra.Command) {
	project := &cobra.Command{
		Use:   "project",
		Short: "Manage the active project",
	}
	project.AddCommand(projectSelectCmd())
	project.AddCommand(projectShowCmd())
	root.AddCommand(project)
	root.AddCommand(loginCmd())
	root.AddCommand(logoutCmd())
}

func projectSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select [project-id]",
		Short: "Set the active project",
		Long: `Set the active project for all subsequent API calls.

Provide a project ID directly to set it without prompting, or omit it to
choose interactively from the list of projects you have access to.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var projectID string

			var projectName, orgName string

			if len(args) == 1 {
				projectID = args[0]
				name, org, err := lookupProjectByID(projectID)
				if err != nil {
					return fmt.Errorf("could not resolve project name: %w", err)
				}
				projectName, orgName = name, org
			} else {
				id, name, org, err := interactiveSelectProject()
				if err != nil {
					return err
				}
				projectID, projectName, orgName = id, name, org
			}

			if err := SaveState(State{ProjectID: projectID, ProjectName: projectName, OrgName: orgName}); err != nil {
				return fmt.Errorf("could not save state: %w", err)
			}

			// Update in-memory profile so the header applies to any further
			// calls within this process (e.g. if someone scripts around this).
			if cfg := configs["apimetrics"]; cfg != nil {
				if p := cfg.Profiles["default"]; p != nil {
					if p.Headers == nil {
						p.Headers = map[string]string{}
					}
					p.Headers["Apimetrics-Project-Id"] = projectID
				}
			}

			var display string
			switch {
			case orgName != "" && projectName != "":
				display = orgName + " / " + projectName
			case projectName != "":
				display = projectName
			default:
				display = projectID
			}
			fmt.Fprintf(Stdout, "Active project set to %s\n", display)
			return nil
		},
	}
}

func projectShowCmd() *cobra.Command {
	var showID bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the active project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			state := loadState()
			if state.ProjectID == "" {
				fmt.Fprintln(Stdout, "No active project set. Run `apimetrics project select` to choose one.")
				return nil
			}
			if showID {
				fmt.Fprintln(Stdout, state.ProjectID)
			} else if state.OrgName != "" && state.ProjectName != "" {
				fmt.Fprintf(Stdout, "%s / %s\n", state.OrgName, state.ProjectName)
			} else if state.ProjectName != "" {
				fmt.Fprintln(Stdout, state.ProjectName)
			} else {
				fmt.Fprintln(Stdout, state.ProjectID)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&showID, "id", false, "Print the project ID instead of the name")
	return cmd
}

// projectEntry is a single item from the list-account-projects response.
type projectEntry struct {
	id      string
	name    string
	orgID   string
	orgName string
}

func interactiveSelectProject() (string, string, string, error) {
	projects, orgs, err := fetchProjects()
	if err != nil {
		return "", "", "", err
	}

	a := defaultAsker{}

	// If projects span more than one org, ask which org first.
	if len(orgs) > 1 {
		orgNames := make([]string, 0, len(orgs))
		for _, n := range orgs {
			orgNames = append(orgNames, n)
		}
		sort.Strings(orgNames)

		chosen := a.askSelect("Select organisation", orgNames, nil, "")

		chosenID := ""
		for id, name := range orgs {
			if name == chosen {
				chosenID = id
				break
			}
		}

		filtered := projects[:0]
		for _, p := range projects {
			if p.orgID == chosenID {
				filtered = append(filtered, p)
			}
		}
		projects = filtered
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].name < projects[j].name
	})

	names := make([]string, len(projects))
	for i, p := range projects {
		names[i] = p.name
	}

	chosen := a.askSelect("Select project", names, nil, "")

	for _, p := range projects {
		if p.name == chosen {
			return p.id, p.name, p.orgName, nil
		}
	}
	return "", "", "", fmt.Errorf("selected project not found")
}

// lookupProjectByID fetches the project list and returns the name for the given ID.
// Returns an empty string (not an error) if the ID is not found.
func lookupProjectByID(id string) (name, orgName string, err error) {
	projects, _, err := fetchProjects()
	if err != nil {
		return "", "", err
	}
	for _, p := range projects {
		if p.id == id {
			return p.name, p.orgName, nil
		}
	}
	return "", "", nil
}

// fetchProjects calls list-account-projects and returns all project entries and the org-id→name map.
func fetchProjects() ([]projectEntry, map[string]string, error) {
	uri, err := operationURI("list-account-projects")
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := GetParsedResponse(req)
	if err != nil {
		return nil, nil, fmt.Errorf("could not list projects: %w", err)
	}
	if resp.Status >= 400 {
		return nil, nil, fmt.Errorf("list-account-projects returned %d", resp.Status)
	}

	body, ok := resp.Body.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected response body type")
	}

	projects, orgs, err := parseProjectsResponse(body)
	if err != nil {
		return nil, nil, err
	}
	if len(projects) == 0 {
		return nil, nil, fmt.Errorf("no projects found")
	}
	return projects, orgs, nil
}

// operationURI returns the URI template for the named operation from the
// already-loaded API spec. Returns an error if the operation is not found.
func operationURI(name string) (string, error) {
	for _, op := range LoadedAPI.Operations {
		if op.Name == name {
			return op.URITemplate, nil
		}
	}
	return "", fmt.Errorf("operation %q not found in loaded API spec", name)
}

// parseProjectsResponse unpacks the body of the list-account-projects
// response into a flat slice of projectEntry values and an org-id→name map.
func parseProjectsResponse(body map[string]any) ([]projectEntry, map[string]string, error) {
	rawProjects, _ := body["projects"].([]any)
	rawOrgs, _ := body["organizations"].(map[string]any)

	orgs := make(map[string]string, len(rawOrgs))
	for id, v := range rawOrgs {
		if org, ok := v.(map[string]any); ok {
			orgs[id], _ = org["name"].(string)
		}
	}
	// Treat a missing org entry as "Personal Projects".
	orgs[""] = "Personal Projects"

	entries := make([]projectEntry, 0, len(rawProjects))
	for _, raw := range rawProjects {
		wrapper, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		proj, ok := wrapper["project"].(map[string]any)
		if !ok {
			continue
		}
		id, _ := proj["id"].(string)
		name, _ := proj["name"].(string)
		orgID, _ := proj["org_id"].(string)
		if id == "" || name == "" {
			continue
		}
		entries = append(entries, projectEntry{id: id, name: name, orgID: orgID, orgName: orgs[orgID]})
	}

	// Only include orgs that actually have projects.
	usedOrgs := map[string]string{}
	for _, e := range entries {
		if n, ok := orgs[e.orgID]; ok {
			usedOrgs[e.orgID] = n
		}
	}

	return entries, usedOrgs, nil
}

func loginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in to APImetrics",
		Long:  "Log in to APImetrics. Commands will automatically trigger login if needed, so this is rarely required.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := configs["apimetrics"]
			if cfg == nil {
				return fmt.Errorf("API configuration not found")
			}
			profile := cfg.Profiles[viper.GetString("rsh-profile")]
			if profile == nil || profile.Auth == nil || profile.Auth.Name == "" {
				return fmt.Errorf("no auth configured for profile %q", viper.GetString("rsh-profile"))
			}
			handler, ok := authHandlers[profile.Auth.Name]
			if !ok {
				return fmt.Errorf("unknown auth handler %q", profile.Auth.Name)
			}

			// Use a dummy request — we only want the side effect of fetching and
			// caching the token, not to actually send a request.
			dummy, _ := http.NewRequest(http.MethodGet, cfg.Base, nil)
			key := "apimetrics:" + viper.GetString("rsh-profile")
			if err := handler.OnRequest(dummy, key, profile.Auth.Params); err != nil {
				return err
			}

			fmt.Fprintln(Stdout, "Logged in successfully.")
			return nil
		},
	}
}

func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out to remove access to APImetrics",
		Long:  "Remove the cached login credentials and active project, forcing re-authentication on the next request.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			key := "apimetrics:default"
			Cache.Set(key+".token", "")
			Cache.Set(key+".refresh", "")
			Cache.Set(key+".type", "")
			Cache.Set(key+".expires", "")

			if err := Cache.WriteConfig(); err != nil {
				return fmt.Errorf("could not write cache: %w", err)
			}

			if err := SaveState(State{}); err != nil {
				return fmt.Errorf("could not clear project state: %w", err)
			}

			// Clear the in-memory profile so any further calls in this process
			// don't send a stale project header.
			if cfg := configs["apimetrics"]; cfg != nil {
				if p := cfg.Profiles["default"]; p != nil {
					delete(p.Headers, "Apimetrics-Project-Id")
				}
			}

			fmt.Fprintln(Stdout, "Logged out.")
			return nil
		},
	}
}
