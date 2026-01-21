package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/iheanyi/agentctl/pkg/output"
	"github.com/iheanyi/agentctl/pkg/sync"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Manage configuration backups",
	Long: `Manage backups of tool configuration files.

agentctl automatically creates backups when syncing configurations.
Use these commands to list, restore, or create backups manually.

Examples:
  agentctl backup list                  # List all backups for all tools
  agentctl backup list --tool claude    # List backups for Claude only
  agentctl backup restore --tool cursor # Restore most recent Cursor backup
  agentctl backup create --tool claude  # Manually create a Claude backup`,
}

var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available backups",
	Long:  `List all available configuration backups for detected tools.`,
	RunE:  runBackupList,
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore from a backup",
	Long: `Restore a tool's configuration from its most recent backup.

This will overwrite the current configuration with the backup.`,
	RunE: runBackupRestore,
}

var backupCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a backup",
	Long:  `Manually create a backup of a tool's configuration.`,
	RunE:  runBackupCreate,
}

var backupTool string

func init() {
	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupRestoreCmd)
	backupCmd.AddCommand(backupCreateCmd)

	backupListCmd.Flags().StringVarP(&backupTool, "tool", "t", "", "Filter to specific tool")
	backupRestoreCmd.Flags().StringVarP(&backupTool, "tool", "t", "", "Tool to restore (required)")
	backupRestoreCmd.MarkFlagRequired("tool")
	backupCreateCmd.Flags().StringVarP(&backupTool, "tool", "t", "", "Tool to backup (required)")
	backupCreateCmd.MarkFlagRequired("tool")
}

// BackupInfo represents information about a single backup
type BackupInfo struct {
	Tool       string    `json:"tool"`
	ConfigPath string    `json:"configPath"`
	BackupPath string    `json:"backupPath"`
	Timestamp  time.Time `json:"timestamp,omitempty"`
	Size       int64     `json:"size"`
}

// BackupListOutput represents the JSON output for backup list
type BackupListOutput struct {
	Backups []BackupInfo `json:"backups"`
	Total   int          `json:"total"`
}

func runBackupList(cmd *cobra.Command, args []string) error {
	var adapters []sync.Adapter

	if backupTool != "" {
		adapter, ok := sync.Get(backupTool)
		if !ok {
			err := fmt.Errorf("unknown tool %q", backupTool)
			if JSONOutput {
				jw := output.NewJSONWriter()
				return jw.WriteError(err)
			}
			return err
		}
		adapters = []sync.Adapter{adapter}
	} else {
		adapters = sync.Detected()
	}

	var allBackups []BackupInfo

	for _, adapter := range adapters {
		configPath := adapter.ConfigPath()
		backups, err := sync.ListBackups(configPath)
		if err != nil {
			if !JSONOutput {
				fmt.Printf("Warning: could not list backups for %s: %v\n", adapter.Name(), err)
			}
			continue
		}

		for _, backupPath := range backups {
			info := BackupInfo{
				Tool:       adapter.Name(),
				ConfigPath: configPath,
				BackupPath: backupPath,
			}

			// Get file info for size and timestamp
			if stat, err := os.Stat(backupPath); err == nil {
				info.Size = stat.Size()
				info.Timestamp = stat.ModTime()
			}

			allBackups = append(allBackups, info)
		}
	}

	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(BackupListOutput{
			Backups: allBackups,
			Total:   len(allBackups),
		})
	}

	if len(allBackups) == 0 {
		fmt.Println("No backups found.")
		return nil
	}

	fmt.Printf("Found %d backup(s):\n\n", len(allBackups))
	for _, backup := range allBackups {
		fmt.Printf("  %s\n", backup.Tool)
		fmt.Printf("    Path: %s\n", backup.BackupPath)
		if !backup.Timestamp.IsZero() {
			fmt.Printf("    Time: %s\n", backup.Timestamp.Format(time.RFC3339))
		}
		fmt.Printf("    Size: %d bytes\n", backup.Size)
		fmt.Println()
	}

	return nil
}

// BackupRestoreOutput represents the JSON output for backup restore
type BackupRestoreOutput struct {
	Tool         string `json:"tool"`
	ConfigPath   string `json:"configPath"`
	RestoredFrom string `json:"restoredFrom"`
}

func runBackupRestore(cmd *cobra.Command, args []string) error {
	adapter, ok := sync.Get(backupTool)
	if !ok {
		err := fmt.Errorf("unknown tool %q", backupTool)
		if JSONOutput {
			jw := output.NewJSONWriter()
			return jw.WriteError(err)
		}
		return err
	}

	configPath := adapter.ConfigPath()

	if !JSONOutput {
		fmt.Printf("Restoring %s configuration from backup...\n", adapter.Name())
	}

	restoredFrom, err := sync.RestoreBackup(configPath)
	if err != nil {
		if JSONOutput {
			jw := output.NewJSONWriter()
			return jw.WriteError(err)
		}
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	if restoredFrom == "" {
		err := fmt.Errorf("no backup found for %s", adapter.Name())
		if JSONOutput {
			jw := output.NewJSONWriter()
			return jw.WriteError(err)
		}
		return err
	}

	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(BackupRestoreOutput{
			Tool:         adapter.Name(),
			ConfigPath:   configPath,
			RestoredFrom: restoredFrom,
		})
	}

	fmt.Printf("✓ Restored from: %s\n", filepath.Base(restoredFrom))
	fmt.Printf("  Config: %s\n", configPath)
	return nil
}

// BackupCreateOutput represents the JSON output for backup create
type BackupCreateOutput struct {
	Tool       string `json:"tool"`
	ConfigPath string `json:"configPath"`
	BackupPath string `json:"backupPath"`
}

func runBackupCreate(cmd *cobra.Command, args []string) error {
	adapter, ok := sync.Get(backupTool)
	if !ok {
		err := fmt.Errorf("unknown tool %q", backupTool)
		if JSONOutput {
			jw := output.NewJSONWriter()
			return jw.WriteError(err)
		}
		return err
	}

	configPath := adapter.ConfigPath()

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		err := fmt.Errorf("no configuration file found for %s at %s", adapter.Name(), configPath)
		if JSONOutput {
			jw := output.NewJSONWriter()
			return jw.WriteError(err)
		}
		return err
	}

	if !JSONOutput {
		fmt.Printf("Creating backup of %s configuration...\n", adapter.Name())
	}

	backupPath, err := sync.CreateBackup(configPath)
	if err != nil {
		if JSONOutput {
			jw := output.NewJSONWriter()
			return jw.WriteError(err)
		}
		return fmt.Errorf("failed to create backup: %w", err)
	}

	if JSONOutput {
		jw := output.NewJSONWriter()
		return jw.WriteSuccess(BackupCreateOutput{
			Tool:       adapter.Name(),
			ConfigPath: configPath,
			BackupPath: backupPath,
		})
	}

	fmt.Printf("✓ Backup created: %s\n", filepath.Base(backupPath))
	return nil
}

// MarshalJSON for BackupListOutput to handle empty slices
func (o BackupListOutput) MarshalJSON() ([]byte, error) {
	type Alias BackupListOutput
	if o.Backups == nil {
		o.Backups = []BackupInfo{}
	}
	return json.Marshal(Alias(o))
}
