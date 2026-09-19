// cmd/room.go — Room Subcommands for Multi-User Collaboration & Sharing
//
// Commands:
//   transfera room create [--name <name>] [--max <limit>] [--port <port>]
//   transfera room join <room-id> [--name <name>]
//   transfera room upload <room-id> <file...> [--user <userId>] [--note <note>]
//   transfera room download <room-id> <file-id> [--dir <output-dir>]
//   transfera room sync <room-id> [--user <userId>]
//   transfera room leave <room-id> --user <userId>

package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/dasanik2001/transfera-client/cli/internal/api"
	"github.com/dasanik2001/transfera-client/cli/internal/validation"
)

var (
	roomCreatorName string
	roomCapacity    int
	roomCustomPort  int
	roomUserName    string
	roomUserId      string
	roomNote        string
	roomOutDir      string
)

var roomCmd = &cobra.Command{
	Use:   "room",
	Short: "Create or join collaborative rooms for multi-user file sharing and chat",
	Long: `Room-based collaboration in Transfera.
Create a room with a custom participant limit, share multiple files simultaneously,
chat with members, and download shared files with zero-RAM streaming.`,
}

var roomCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new collaborative room",
	Example: `  transfera room create --name Alice --max 5
  transfera room create --name "Dev Team" --max 10 --port 54321`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient(apiBaseURL, verbose)
		resp, err := client.CreateRoom(roomCreatorName, roomCapacity, roomCustomPort)
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("  ✓ Room created successfully!")
		fmt.Printf("    Room ID (Port) : %d\n", resp.RoomID)
		fmt.Printf("    Host           : %s\n", roomCreatorName)
		fmt.Printf("    User ID        : %s\n", resp.UserID)
		fmt.Printf("    Max Capacity   : %d participants\n", resp.MaxParticipants)
		fmt.Println()
		fmt.Printf("  Share the Room ID %d with others so they can join via:\n", resp.RoomID)
		fmt.Printf("    transfera room join %d --name <YourName>\n\n", resp.RoomID)
		return nil
	},
}

var roomJoinCmd = &cobra.Command{
	Use:   "join <room-id>",
	Short: "Join an existing collaboration room",
	Args:  cobra.ExactArgs(1),
	Example: `  transfera room join 52341 --name Bob`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid room ID: %s (must be a port between 1 and 65535)", args[0])
		}

		client := api.NewClient(apiBaseURL, verbose)
		resp, err := client.JoinRoom(port, roomUserName)
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("  ✓ Joined room successfully!")
		fmt.Printf("    Room ID      : %d\n", resp.RoomID)
		fmt.Printf("    Display Name : %s\n", roomUserName)
		fmt.Printf("    User ID      : %s\n", resp.UserID)
		fmt.Printf("    Max Capacity : %d participants\n\n", resp.MaxParticipants)
		return nil
	},
}

var roomUploadCmd = &cobra.Command{
	Use:   "upload <room-id> <file...>",
	Short: "Upload one or more files to a room",
	Args:  cobra.MinimumNArgs(2),
	Example: `  transfera room upload 52341 report.pdf dataset.zip --note "Project docs"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid room ID: %s", args[0])
		}

		files := args[1:]
		for _, f := range files {
			if _, err := os.Stat(f); err != nil {
				return fmt.Errorf("file not found: %s", f)
			}
		}

		client := api.NewClient(apiBaseURL, verbose)
		fmt.Printf("\n  Uploading %d file(s) to room %d...\n", len(files), port)

		if err := client.UploadRoomFiles(port, roomUserId, files, roomNote); err != nil {
			return err
		}

		fmt.Println("  ✓ Files uploaded and shared in room successfully!")
		for _, f := range files {
			fmt.Printf("    • %s\n", f)
		}
		fmt.Println()
		return nil
	},
}

var roomDownloadCmd = &cobra.Command{
	Use:   "download <room-id> <file-id>",
	Short: "Download a shared file from a room",
	Args:  cobra.ExactArgs(2),
	Example: `  transfera room download 52341 fil_a1b2c3d4 -o ./downloads/`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid room ID: %s", args[0])
		}
		fileId := args[1]

		targetDir := roomOutDir
		if targetDir == "" {
			targetDir = "."
		}

		client := api.NewClient(apiBaseURL, verbose)
		fmt.Printf("\n  Downloading file %s from room %d...\n", fileId, port)

		savedPath, err := client.DownloadRoomFile(port, fileId, targetDir)
		if err != nil {
			return err
		}

		fi, _ := os.Stat(savedPath)
		var sizeStr string
		if fi != nil {
			sizeStr = validation.FormatFileSize(fi.Size())
		}

		fmt.Println("  ✓ File downloaded successfully!")
		fmt.Printf("    Saved to : %s\n", savedPath)
		if sizeStr != "" {
			fmt.Printf("    Size     : %s\n", sizeStr)
		}
		fmt.Println()
		return nil
	},
}

var roomSyncCmd = &cobra.Command{
	Use:   "sync <room-id>",
	Short: "View live messages, members, and files in a room",
	Args:  cobra.ExactArgs(1),
	Example: `  transfera room sync 52341`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid room ID: %s", args[0])
		}

		client := api.NewClient(apiBaseURL, verbose)
		sync, err := client.SyncRoom(port, roomUserId, 0)
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Printf("  === Room %d ===\n", sync.RoomID)
		fmt.Printf("  Capacity: %d / %d members\n", len(sync.Participants), sync.MaxParticipants)
		fmt.Println("  Members:")
		for _, p := range sync.Participants {
			role := ""
			if p.IsCreator {
				role = " (Host)"
			}
			fmt.Printf("    • %s%s (ID: %s)\n", p.Name, role, p.ID)
		}

		fmt.Println("\n  Shared Files:")
		if len(sync.Files) == 0 {
			fmt.Println("    (No files uploaded yet)")
		} else {
			for _, f := range sync.Files {
				fmt.Printf("    • [%s] %s (%s) — uploaded by %s\n",
					f.ID, f.Name, validation.FormatFileSize(f.Size), f.UploaderName)
			}
		}

		fmt.Println("\n  Chat / Event Log:")
		if len(sync.Messages) == 0 {
			fmt.Println("    (No messages yet)")
		} else {
			for _, m := range sync.Messages {
				t := time.UnixMilli(m.Timestamp).Format("15:04:05")
				if m.Type == "system" {
					fmt.Printf("    [%s] * %s\n", t, m.Text)
				} else if m.Type == "file" {
					fmt.Printf("    [%s] %s shared file: %s (ID: %s)\n", t, m.SenderName, m.FileName, m.FileID)
				} else {
					fmt.Printf("    [%s] %s: %s\n", t, m.SenderName, m.Text)
				}
			}
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(roomCmd)

	// Subcommands
	roomCmd.AddCommand(roomCreateCmd)
	roomCmd.AddCommand(roomJoinCmd)
	roomCmd.AddCommand(roomUploadCmd)
	roomCmd.AddCommand(roomDownloadCmd)
	roomCmd.AddCommand(roomSyncCmd)

	// Flags
	roomCreateCmd.Flags().StringVar(&roomCreatorName, "name", "Host", "Your display name as room creator")
	roomCreateCmd.Flags().IntVar(&roomCapacity, "max", 5, "Maximum participants allowed in the room (2-100)")
	roomCreateCmd.Flags().IntVar(&roomCustomPort, "port", 0, "Custom room ID / port (default auto-assigns 49152-65535)")

	roomJoinCmd.Flags().StringVar(&roomUserName, "name", "Guest", "Your display name")

	roomUploadCmd.Flags().StringVar(&roomUserId, "user", "", "Your user ID (optional)")
	roomUploadCmd.Flags().StringVar(&roomNote, "note", "", "Optional message/note accompanying the files")

	roomDownloadCmd.Flags().StringVarP(&roomOutDir, "output", "o", "", "Directory to save downloaded file (default: current directory)")

	roomSyncCmd.Flags().StringVar(&roomUserId, "user", "", "Your user ID (optional)")
}
