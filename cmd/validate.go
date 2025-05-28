package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devarispbrown/kc2con/internal/migration"
	"github.com/spf13/cobra"
)

var (
	conduitConfig string
	strictMode    bool
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate Conduit configurations",
	Long: `Validate generated Conduit pipeline configurations for correctness.

This command will:
- Check YAML syntax and structure
- Validate connector configurations
- Verify processor chains
- Check for common configuration issues
- Ensure all references are valid`,
	Example: `  # Validate a single pipeline
  kc2con validate --conduit-config ./pipeline.yaml

  # Validate all configs in directory
  kc2con validate --config-dir ./conduit-configs

  # Strict validation mode
  kc2con validate --config-dir ./conduit-configs --strict`,
	RunE: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)

	validateCmd.Flags().StringVar(&conduitConfig, "conduit-config", "", "Path to Conduit pipeline configuration file")
	validateCmd.Flags().StringVarP(&configDir, "config-dir", "d", "", "Directory containing Conduit configurations")
	validateCmd.Flags().BoolVar(&strictMode, "strict", false, "Enable strict validation mode")
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Create styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#50FA7B"))

	warningStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFB86C"))

	errorStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF5F87"))

	fmt.Println(headerStyle.Render("🔍 Conduit Configuration Validator"))
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	// Create validator
	validator := migration.NewValidator(strictMode)

	// Determine what to validate
	var filesToValidate []string

	if conduitConfig != "" {
		// Validate single file
		filesToValidate = []string{conduitConfig}
	} else if configDir != "" {
		// Find all YAML files in directory
		err := filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".yaml" || ext == ".yml" {
					filesToValidate = append(filesToValidate, path)
				}
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to scan directory: %w", err)
		}
	} else {
		return fmt.Errorf("either --conduit-config or --config-dir must be specified")
	}

	if len(filesToValidate) == 0 {
		fmt.Println(warningStyle.Render("⚠️  No configuration files found to validate"))
		return nil
	}

	fmt.Printf("📁 Found %d file(s) to validate\n", len(filesToValidate))
	if strictMode {
		fmt.Println("🔒 Strict mode enabled")
	}
	fmt.Println()

	// Track overall results
	totalFiles := len(filesToValidate)
	validFiles := 0
	filesWithWarnings := 0
	filesWithErrors := 0

	// Validate each file
	for _, file := range filesToValidate {
		relPath := file
		if configDir != "" {
			relPath, _ = filepath.Rel(configDir, file)
		}

		fmt.Printf("Validating: %s\n", relPath)

		result, err := validator.ValidateFile(file)
		if err != nil {
			fmt.Printf("  %s Failed to parse: %v\n", errorStyle.Render("❌"), err)
			filesWithErrors++
			continue
		}

		// Display results
		if result.Valid && len(result.Warnings) == 0 {
			fmt.Printf("  %s Valid\n", successStyle.Render("✅"))
			validFiles++
		} else if result.Valid && len(result.Warnings) > 0 {
			fmt.Printf("  %s Valid with warnings\n", warningStyle.Render("⚠️"))
			validFiles++
			filesWithWarnings++

			// Show warnings
			for _, warning := range result.Warnings {
				fmt.Printf("    %s %s", warningStyle.Render("⚠️"), warning.Message)
				if warning.Path != "" {
					fmt.Printf(" (at %s)", warning.Path)
				}
				fmt.Println()
			}
		} else {
			fmt.Printf("  %s Invalid\n", errorStyle.Render("❌"))
			filesWithErrors++

			// Show errors
			for _, err := range result.Errors {
				fmt.Printf("    %s %s", errorStyle.Render("❌"), err.Message)
				if err.Path != "" {
					fmt.Printf(" (at %s)", err.Path)
				}
				fmt.Println()
			}

			// Show warnings too
			for _, warning := range result.Warnings {
				fmt.Printf("    %s %s", warningStyle.Render("⚠️"), warning.Message)
				if warning.Path != "" {
					fmt.Printf(" (at %s)", warning.Path)
				}
				fmt.Println()
			}
		}

		fmt.Println()
	}

	// Summary
	fmt.Println(headerStyle.Render("📊 Validation Summary"))
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("Total files: %d\n", totalFiles)
	fmt.Printf("%s Valid: %d\n", successStyle.Render("✅"), validFiles)
	if filesWithWarnings > 0 {
		fmt.Printf("%s With warnings: %d\n", warningStyle.Render("⚠️"), filesWithWarnings)
	}
	if filesWithErrors > 0 {
		fmt.Printf("%s With errors: %d\n", errorStyle.Render("❌"), filesWithErrors)
	}

	// Exit code
	if filesWithErrors > 0 {
		return fmt.Errorf("validation failed: %d file(s) have errors", filesWithErrors)
	}

	if strictMode && filesWithWarnings > 0 {
		return fmt.Errorf("validation failed in strict mode: %d file(s) have warnings", filesWithWarnings)
	}

	fmt.Println()
	fmt.Println(successStyle.Render("✨ All validations passed!"))

	return nil
}
