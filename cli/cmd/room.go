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
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dasanik2001/transfera-client/cli/internal/api"
	"github.com/dasanik2001/transfera-client/cli/internal/tui"
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
	roomInteractive bool
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
  transfera room create --name "Dev Team" --max 10 --port 54321
  transfera room create --name Alice -i   (enter live interactive session)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.NewClient(apiBaseURL, verbose)
		resp, err := client.CreateRoom(roomCreatorName, roomCapacity, roomCustomPort)
		if err != nil {
			return err
		}

		if roomInteractive {
			tui.RunRoomSession(client, resp.RoomID, resp.UserID, roomCreatorName, true, resp.MaxParticipants)
			return nil
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
		fmt.Println("  To enter the interactive live room session:")
		fmt.Printf("    transfera room session %d --user %s --name \"%s\"\n\n", resp.RoomID, resp.UserID, roomCreatorName)
		return nil
	},
}

var roomJoinCmd = &cobra.Command{
	Use:   "join <room-id>",
	Short: "Join an existing collaboration room",
	Args:  cobra.ExactArgs(1),
	Example: `  transfera room join 52341 --name Bob
  transfera room join 52341 --name Bob -i   (enter live interactive session)`,
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

		if roomInteractive {
			tui.RunRoomSession(client, resp.RoomID, resp.UserID, roomUserName, false, resp.MaxParticipants)
			return nil
		}

		fmt.Println()
		fmt.Println("  ✓ Joined room successfully!")
		fmt.Printf("    Room ID      : %d\n", resp.RoomID)
		fmt.Printf("    Display Name : %s\n", roomUserName)
		fmt.Printf("    User ID      : %s\n", resp.UserID)
		fmt.Printf("    Max Capacity : %d participants\n\n", resp.MaxParticipants)
		fmt.Println("  To enter the interactive live room session:")
		fmt.Printf("    transfera room session %d --user %s --name \"%s\"\n\n", resp.RoomID, resp.UserID, roomUserName)
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

var roomRemoveCmd = &cobra.Command{
	Use:     "remove <room-id> <target-user-id>",
	Aliases: []string{"kick"},
	Short:   "Remove/kick a participant from a room",
	Args:    cobra.ExactArgs(2),
	Example: `  transfera room remove 52341 usr_abc123 --user usr_host`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid room ID: %s (must be a port between 1 and 65535)", args[0])
		}
		targetUserId := args[1]

		client := api.NewClient(apiBaseURL, verbose)
		if err := client.RemoveRoomParticipant(port, roomUserId, targetUserId); err != nil {
			return err
		}

		fmt.Println()
		fmt.Printf("  ✓ Participant %s was removed from room %d\n\n", targetUserId, port)
		return nil
	},
}

var roomSessionCmd = &cobra.Command{
	Use:   "session <room-id>",
	Short: "Enter interactive collaboration session for a room",
	Args:  cobra.ExactArgs(1),
	Example: `  transfera room session 52341 --user usr_abc123 --name Alice`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid room ID: %s (must be a port between 1 and 65535)", args[0])
		}
		if roomUserId == "" {
			return fmt.Errorf("--user <userId> is required to connect to room session")
		}

		displayName := roomUserName
		if displayName == "" {
			displayName = "Member"
		}

		client := api.NewClient(apiBaseURL, verbose)
		sync, err := client.SyncRoom(port, roomUserId, 0)
		if err != nil {
			return fmt.Errorf("failed to connect to room %d: %w", port, err)
		}

		isCreator := false
		for _, p := range sync.Participants {
			if p.ID == roomUserId {
				isCreator = p.IsCreator
				if roomUserName == "" || roomUserName == "Member" {
					displayName = p.Name
				}
				break
			}
		}

		tui.RunRoomSession(client, port, roomUserId, displayName, isCreator, sync.MaxParticipants)
		return nil
	},
}

var roomMessageCmd = &cobra.Command{
	Use:     "message <room-id> <text...>",
	Aliases: []string{"msg"},
	Short:   "Send a text message to a room",
	Args:    cobra.MinimumNArgs(2),
	Example: `  transfera room message 52341 Hello team! --user usr_abc123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid room ID: %s", args[0])
		}
		if roomUserId == "" {
			return fmt.Errorf("--user <userId> is required to send messages")
		}

		text := strings.Join(args[1:], " ")
		client := api.NewClient(apiBaseURL, verbose)
		if err := client.SendRoomMessage(port, roomUserId, text); err != nil {
			return err
		}

		fmt.Println("  ✓ Message sent successfully!")
		return nil
	},
}

var roomLeaveCmd = &cobra.Command{
	Use:   "leave <room-id>",
	Short: "Leave a collaborative room",
	Args:  cobra.ExactArgs(1),
	Example: `  transfera room leave 52341 --user usr_abc123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid room ID: %s", args[0])
		}
		if roomUserId == "" {
			return fmt.Errorf("--user <userId> is required to leave a room")
		}

		client := api.NewClient(apiBaseURL, verbose)
		if err := client.LeaveRoom(port, roomUserId); err != nil {
			return err
		}

		fmt.Printf("\n  ✓ Left room %d successfully!\n\n", port)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(roomCmd)

	// Subcommands
	roomCmd.AddCommand(roomCreateCmd)
	roomCmd.AddCommand(roomJoinCmd)
	roomCmd.AddCommand(roomSessionCmd)
	roomCmd.AddCommand(roomMessageCmd)
	roomCmd.AddCommand(roomUploadCmd)
	roomCmd.AddCommand(roomDownloadCmd)
	roomCmd.AddCommand(roomSyncCmd)
	roomCmd.AddCommand(roomRemoveCmd)
	roomCmd.AddCommand(roomLeaveCmd)

	// Flags
	roomCreateCmd.Flags().StringVar(&roomCreatorName, "name", "Host", "Your display name as room creator")
	roomCreateCmd.Flags().IntVar(&roomCapacity, "max", 5, "Maximum participants allowed in the room (2-100)")
	roomCreateCmd.Flags().IntVar(&roomCustomPort, "port", 0, "Custom room ID / port (default auto-assigns 49152-65535)")
	roomCreateCmd.Flags().BoolVarP(&roomInteractive, "interactive", "i", false, "Enter interactive collaboration session immediately")

	roomJoinCmd.Flags().StringVar(&roomUserName, "name", "Guest", "Your display name")
	roomJoinCmd.Flags().BoolVarP(&roomInteractive, "interactive", "i", false, "Enter interactive collaboration session immediately")

	roomSessionCmd.Flags().StringVar(&roomUserId, "user", "", "Your user ID in the room (required)")
	roomSessionCmd.Flags().StringVar(&roomUserName, "name", "", "Your display name (optional)")

	roomMessageCmd.Flags().StringVar(&roomUserId, "user", "", "Your user ID (required)")

	roomUploadCmd.Flags().StringVar(&roomUserId, "user", "", "Your user ID (optional)")
	roomUploadCmd.Flags().StringVar(&roomNote, "note", "", "Optional message/note accompanying the files")

	roomDownloadCmd.Flags().StringVarP(&roomOutDir, "output", "o", "", "Directory to save downloaded file (default: current directory)")

	roomSyncCmd.Flags().StringVar(&roomUserId, "user", "", "Your user ID (optional)")
	roomRemoveCmd.Flags().StringVar(&roomUserId, "user", "", "Your user ID (optional)")
	roomLeaveCmd.Flags().StringVar(&roomUserId, "user", "", "Your user ID (required)")
}
