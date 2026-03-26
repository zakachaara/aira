package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aira/aira/internal/delivery"
	"github.com/aira/aira/internal/models"
)

func sourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "List and manage configured RSS feeds",
		Long:  `Manage the RSS feed sources AIRA monitors. Supports add, list, remove, enable, and disable.`,
	}

	cmd.AddCommand(
		sourcesListCmd(),
		sourcesAddCmd(),
		sourcesRemoveCmd(),
		sourcesEnableCmd(),
		sourcesDisableCmd(),
		sourcesInitDefaultsCmd(),
	)
	return cmd
}

func sourcesListCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List configured sources",
		Example: `  aira sources list
  aira sources list --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := getApp(cmd)
			sources, err := app.repo.ListSources(cmd.Context(), !all)
			if err != nil {
				return err
			}
			fmt.Println()
			delivery.PrintSources(os.Stdout, sources)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "include disabled sources")
	return cmd
}

func sourcesAddCmd() *cobra.Command {
	var name, url, category string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new RSS source",
		Example: `  aira sources add --name "arXiv AI" --url "https://arxiv.org/rss/cs.AI" --category ai_research
  aira sources add --name "CNCF Blog" --url "https://www.cncf.io/feed/" --category cloud_native`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := getApp(cmd)
			if name == "" || url == "" {
				return fmt.Errorf("--name and --url are required")
			}
			cat := models.Category(category)
			if !validCategory(cat) {
				cat = models.CategoryUncategorized
			}
			s := &models.Source{
				Name:     name,
				URL:      url,
				Category: cat,
				Active:   true,
			}
			if err := app.repo.SaveSource(cmd.Context(), s); err != nil {
				return fmt.Errorf("saving source: %w", err)
			}
			fmt.Printf("\n  ✓ Source %q added (id=%d, category=%s)\n\n", s.Name, s.ID, s.Category)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "display name for the source (required)")
	cmd.Flags().StringVar(&url, "url", "", "RSS/Atom feed URL (required)")
	cmd.Flags().StringVar(&category, "category", "uncategorized",
		"category hint: ai_research|model_release|ai_infrastructure|cloud_native")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

func sourcesRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <id>",
		Aliases: []string{"rm", "delete"},
		Short:   "Disable a source by ID",
		Args:    cobra.ExactArgs(1),
		Example: `  aira sources remove 3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid source id %q", args[0])
			}
			app := getApp(cmd)
			if err := app.repo.DeleteSource(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Printf("\n  ✓ Source %d disabled.\n\n", id)
			return nil
		},
	}
}

func sourcesEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <id>",
		Short: "Re-enable a disabled source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid id %q", args[0])
			}
			app := getApp(cmd)
			src, err := app.repo.GetSource(cmd.Context(), id)
			if err != nil {
				return err
			}
			src.Active = true
			if err := app.repo.SaveSource(cmd.Context(), src); err != nil {
				return err
			}
			fmt.Printf("\n  ✓ Source %d (%s) enabled.\n\n", id, src.Name)
			return nil
		},
	}
}

func sourcesDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable an active source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sourcesRemoveCmd().RunE(cmd, args) // reuses same logic
		},
	}
}

// sourcesInitDefaultsCmd seeds a curated set of default sources.
func sourcesInitDefaultsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init-defaults",
		Short: "Seed the database with a curated set of default sources",
		Long: `Adds a hand-picked collection of high-quality RSS feeds covering:
  • AI Research  : arXiv (cs.AI, cs.LG, cs.CL, cs.CV), Papers with Code
  • Model Release: OpenAI, Anthropic, Google DeepMind, Mistral, HuggingFace
  • AI Infra     : vLLM, LangChain, LlamaIndex blogs
  • Cloud Native : CNCF Blog, Kubernetes Blog, eBPF.io`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := getApp(cmd)
			ctx := cmd.Context()
			added := 0
			for _, s := range defaultSources() {
				src := s // copy
				if err := app.repo.SaveSource(ctx, &src); err != nil {
					fmt.Fprintf(os.Stderr, "  ⚠  skipping %q: %v\n", src.Name, err)
					continue
				}
				added++
			}
			fmt.Printf("\n  ✓ Added %d default sources.\n", added)
			fmt.Println("  Run: aira sources list")
			return nil
		},
	}
}

func defaultSources() []models.Source {
	return []models.Source{
		// arXiv
		{Name: "arXiv cs.AI", URL: "https://rss.arxiv.org/rss/cs.AI", Category: models.CategoryAIResearch, Active: true},
		{Name: "arXiv cs.LG", URL: "https://rss.arxiv.org/rss/cs.LG", Category: models.CategoryAIResearch, Active: true},
		{Name: "arXiv cs.CL", URL: "https://rss.arxiv.org/rss/cs.CL", Category: models.CategoryAIResearch, Active: true},
		{Name: "arXiv cs.CV", URL: "https://rss.arxiv.org/rss/cs.CV", Category: models.CategoryAIResearch, Active: true},
		{Name: "Papers With Code", URL: "https://paperswithcode.com/latest.rss", Category: models.CategoryAIResearch, Active: true},
		// Research blogs
		{Name: "OpenAI Blog", URL: "https://openai.com/blog/rss.xml", Category: models.CategoryModelRelease, Active: true},
		{Name: "Anthropic News", URL: "https://www.anthropic.com/news.rss", Category: models.CategoryModelRelease, Active: true},
		{Name: "Google DeepMind Blog", URL: "https://deepmind.google/blog/rss.xml", Category: models.CategoryAIResearch, Active: true},
		{Name: "Mistral AI Blog", URL: "https://mistral.ai/news/rss.xml", Category: models.CategoryModelRelease, Active: true},
		{Name: "Meta AI Blog", URL: "https://ai.meta.com/blog/rss/", Category: models.CategoryAIResearch, Active: true},
		{Name: "Hugging Face Blog", URL: "https://huggingface.co/blog/feed.xml", Category: models.CategoryModelRelease, Active: true},
		// AI Infrastructure
		{Name: "LangChain Blog", URL: "https://blog.langchain.dev/rss/", Category: models.CategoryAIInfrastructure, Active: true},
		{Name: "LlamaIndex Blog", URL: "https://www.llamaindex.ai/blog/rss", Category: models.CategoryAIInfrastructure, Active: true},
		// Cloud Native
		{Name: "CNCF Blog", URL: "https://www.cncf.io/feed/", Category: models.CategoryCloudNative, Active: true},
		{Name: "Kubernetes Blog", URL: "https://kubernetes.io/feed.xml", Category: models.CategoryCloudNative, Active: true},
		{Name: "The New Stack", URL: "https://thenewstack.io/feed/", Category: models.CategoryCloudNative, Active: true},
	}
}

func validCategory(c models.Category) bool {
	switch c {
	case models.CategoryAIResearch, models.CategoryModelRelease,
		models.CategoryAIInfrastructure, models.CategoryCloudNative,
		models.CategoryUncategorized:
		return true
	}
	return false
}
