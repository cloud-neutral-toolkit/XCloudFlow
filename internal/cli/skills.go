package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"xcloudflow/internal/skills"
	"xcloudflow/internal/store"
)

func skillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Skills: list/read external skills (local/http) and optionally cache in PostgreSQL",
	}
	cmd.AddCommand(skillsListCmd())
	cmd.AddCommand(skillsSourceAddCmd())
	cmd.AddCommand(skillsSyncCmd())
	return cmd
}

func skillsListCmd() *cobra.Command {
	var dir string
	var show bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List skills under a directory (expects <dir>/*/SKILL.md)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				dir = "skills"
			}
			found, err := skills.DiscoverLocal(dir)
			if err != nil {
				return err
			}
			if show {
				for _, s := range found {
					full := filepath.Join(dir, s.Name, "SKILL.md")
					doc, err := skills.ReadSkill(full)
					if err != nil {
						return err
					}
					fmt.Printf("## %s (%s)\n\n%s\n\n", doc.Name, doc.SHA256, doc.Content)
				}
				return nil
			}
			b, _ := json.MarshalIndent(found, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "skills", "skills directory to scan")
	cmd.Flags().BoolVar(&show, "show", false, "print SKILL.md content")
	return cmd
}

func skillsSourceAddCmd() *cobra.Command {
	var name, typ, uri, ref, basePath string
	var enabled bool
	cmd := &cobra.Command{
		Use:   "add-source",
		Short: "Upsert an external skill source into PostgreSQL (xcf.skill_sources)",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			if name == "" || typ == "" || uri == "" {
				return fmt.Errorf("missing required: --name --type --uri")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()
			_, err = st.AddSkillSource(ctx, store.SkillSource{
				Name:     name,
				Type:     typ,
				URI:      uri,
				Ref:      ref,
				BasePath: basePath,
				Enabled:  enabled,
			})
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "source name (unique)")
	cmd.Flags().StringVar(&typ, "type", "", "source type: local|http")
	cmd.Flags().StringVar(&uri, "uri", "", "source URI (path or http url)")
	cmd.Flags().StringVar(&ref, "ref", "", "reserved (git ref) - not used today")
	cmd.Flags().StringVar(&basePath, "path", "", "base path inside source (optional)")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable this source")
	return cmd
}

func skillsSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch skills from configured sources and cache into PostgreSQL (xcf.skill_docs)",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn, err := dsnOrErr()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			st, err := store.Open(ctx, dsn)
			if err != nil {
				return err
			}
			defer st.Close()

			srcs, err := st.ListSkillSources(ctx)
			if err != nil {
				return err
			}
			for _, src := range srcs {
				if !src.Enabled {
					continue
				}
				switch src.Type {
				case "local":
					dir := src.URI
					if src.BasePath != "" {
						dir = filepath.Join(dir, src.BasePath)
					}
					found, err := skills.DiscoverLocal(dir)
					if err != nil {
						return fmt.Errorf("source %s: %w", src.Name, err)
					}
					for _, sk := range found {
						p := filepath.Join(dir, sk.Name, "SKILL.md")
						doc, err := skills.ReadSkill(p)
						if err != nil {
							return err
						}
						if err := st.UpsertSkillDoc(ctx, src.SourceID, filepath.Join(sk.Name, "SKILL.md"), doc.SHA256, doc.Content); err != nil {
							return err
						}
					}
				case "http":
					doc, err := skills.FetchHTTP(src.URI, 15*time.Second)
					if err != nil {
						return fmt.Errorf("source %s: %w", src.Name, err)
					}
					if err := st.UpsertSkillDoc(ctx, src.SourceID, "SKILL.md", doc.SHA256, doc.Content); err != nil {
						return err
					}
				default:
					return fmt.Errorf("unsupported source type (today): %s", src.Type)
				}
			}
			fmt.Println("ok: skills synced")
			return nil
		},
	}
	return cmd
}

