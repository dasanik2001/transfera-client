// internal/tui/room_session.go — Interactive Terminal Session for Collaborative Rooms
//
// Provides a live, real-time terminal interface for room participants:
//   - Continuous background synchronization (chat, member updates, file shares)
//   - Real-time text messaging
//   - In-room commands: /upload, /download, /files, /members, /kick, /leave, /clear, /help

package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dasanik2001/transfera-client/cli/internal/api"
	"github.com/dasanik2001/transfera-client/cli/internal/validation"
)

// RunRoomSession runs an interactive collaborative room session in the terminal.
func RunRoomSession(client *api.Client, roomID int, userID string, userName string, isCreator bool, maxCapacity int) {
	fmt.Print("\033[2J\033[H") // Clear screen

	printRoomHeader(roomID, userName, userID, isCreator, maxCapacity)

	seenMessages := make(map[string]bool)
	var mu sync.Mutex
	stopChan := make(chan struct{})
	var currentParticipants []api.RoomParticipant
	var currentFiles []api.RoomFile

	// Initial sync to display existing history
	initialSync, err := client.SyncRoom(roomID, userID, 0)
	if err == nil && initialSync != nil {
		mu.Lock()
		currentParticipants = initialSync.Participants
		currentFiles = initialSync.Files
		for _, m := range initialSync.Messages {
			seenMessages[m.ID] = true
			printRoomMessage(m)
		}
		mu.Unlock()
	} else if err != nil {
		fmt.Printf("  %s⚠ Warning: Failed initial room sync: %s%s\n\n", colorYellow, err, colorReset)
	}

	// Background polling goroutine for live updates
	go func() {
		ticker := time.NewTicker(1500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stopChan:
				return
			case <-ticker.C:
				syncResp, err := client.SyncRoom(roomID, userID, 0)
				if err != nil {
					continue
				}

				mu.Lock()
				currentParticipants = syncResp.Participants
				currentFiles = syncResp.Files

				newCount := 0
				for _, m := range syncResp.Messages {
					if !seenMessages[m.ID] {
						seenMessages[m.ID] = true
						if newCount == 0 {
							// Erase current input prompt line
							fmt.Print("\r\033[K")
						}
						printRoomMessage(m)
						newCount++
					}
				}

				if newCount > 0 {
					// Re-display prompt
					fmt.Printf("  %s[%s]>%s ", colorCyan, userName, colorReset)
				}
				mu.Unlock()
			}
		}
	}()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("  %s[%s]>%s ", colorCyan, userName, colorReset)
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}

		// Handle slash commands
		if strings.HasPrefix(text, "/") {
			parts := strings.Fields(text)
			cmd := strings.ToLower(parts[0])

			switch cmd {
			case "/help", "/h", "/?":
				printRoomHelp(isCreator)

			case "/clear", "/cls":
				fmt.Print("\033[2J\033[H")
				printRoomHeader(roomID, userName, userID, isCreator, maxCapacity)

			case "/members", "/who", "/users":
				mu.Lock()
				partsCopy := make([]api.RoomParticipant, len(currentParticipants))
				copy(partsCopy, currentParticipants)
				mu.Unlock()
				printParticipants(partsCopy, userID)

			case "/files", "/list", "/ls":
				mu.Lock()
				filesCopy := make([]api.RoomFile, len(currentFiles))
				copy(filesCopy, currentFiles)
				mu.Unlock()
				printFiles(filesCopy)

			case "/upload", "/send":
				if len(parts) < 2 {
					fmt.Printf("  %sUsage: /upload <file-path...> [--note <note>]%s\n", colorYellow, colorReset)
					continue
				}
				handleInRoomUpload(client, roomID, userID, parts[1:])

			case "/download", "/get":
				if len(parts) < 2 {
					fmt.Printf("  %sUsage: /download <file-id> [save-directory]%s\n", colorYellow, colorReset)
					continue
				}
				fileID := parts[1]
				saveDir := "."
				if len(parts) >= 3 {
					saveDir = cleanPath(parts[2])
				}
				handleInRoomDownload(client, roomID, fileID, saveDir)

			case "/kick", "/remove":
				if !isCreator {
					fmt.Printf("  %sOnly the room host can kick participants.%s\n", colorRed, colorReset)
					continue
				}
				if len(parts) < 2 {
					fmt.Printf("  %sUsage: /kick <target-user-id>%s\n", colorYellow, colorReset)
					continue
				}
				targetID := parts[1]
				if targetID == userID {
					fmt.Printf("  %sYou cannot kick yourself. Use /leave to exit.%s\n", colorRed, colorReset)
					continue
				}
				if err := client.RemoveRoomParticipant(roomID, userID, targetID); err != nil {
					fmt.Printf("  %sFailed to kick participant: %s%s\n", colorRed, err, colorReset)
				} else {
					fmt.Printf("  %s✓ Participant %s removed.%s\n", colorGreen, targetID, colorReset)
				}

			case "/leave", "/exit", "/quit", "/q":
				close(stopChan)
				fmt.Printf("\n  %sLeaving room %d...%s\n", colorDim, roomID, colorReset)
				_ = client.LeaveRoom(roomID, userID)
				fmt.Printf("  %s✓ You have left the room.%s\n\n", colorGreen, colorReset)
				return

			default:
				fmt.Printf("  %sUnknown command '%s'. Type /help for available commands.%s\n", colorYellow, cmd, colorReset)
			}
			continue
		}

		// Send regular chat message
		if err := client.SendRoomMessage(roomID, userID, text); err != nil {
			fmt.Printf("  %sFailed to send message: %s%s\n", colorRed, err, colorReset)
		}
	}

	close(stopChan)
	_ = client.LeaveRoom(roomID, userID)
}

func printRoomHeader(roomID int, userName string, userID string, isCreator bool, maxCapacity int) {
	role := "Participant"
	roleColor := colorCyan
	if isCreator {
		role = "Host 👑"
		roleColor = colorYellow
	}

	divider := strings.Repeat("─", 68)

	fmt.Println()
	fmt.Printf("  %s%s%s%s\n", colorCyan, colorBold, divider, colorReset)
	fmt.Printf("  %s%s  TRANSFERA COLLABORATION ROOM%s\n", colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s%s%s\n", colorCyan, colorBold, divider, colorReset)
	fmt.Printf("    %s●%s Room ID (Port) : %s%s%d%s\n", colorCyan, colorReset, colorBold, colorGreen, roomID, colorReset)
	fmt.Printf("    %s●%s Your Name      : %s%s%s\n", colorCyan, colorReset, colorBold, userName, colorReset)
	fmt.Printf("    %s●%s Your Role      : %s%s%s\n", colorCyan, colorReset, roleColor, role, colorReset)
	fmt.Printf("    %s●%s Your User ID   : %s%s%s\n", colorCyan, colorReset, colorDim, userID, colorReset)
	fmt.Printf("    %s●%s Max Capacity   : %d participants\n", colorCyan, colorReset, maxCapacity)
	fmt.Printf("  %s%s%s%s\n", colorCyan, colorDim, divider, colorReset)
	fmt.Printf("    💬 Type a message and press Enter to chat\n")
	fmt.Printf("    📁 %s/upload <file>%s     Share files in the room\n", colorBold, colorReset)
	fmt.Printf("    ❓ %s/help%s              Show all in-room commands\n", colorBold, colorReset)
	fmt.Printf("    🚪 %s/leave%s             Exit this room\n", colorBold, colorReset)
	fmt.Printf("  %s%s%s%s\n\n", colorCyan, colorBold, divider, colorReset)
}

func printRoomHelp(isCreator bool) {
	fmt.Println()
	fmt.Printf("  %s%sIn-Room Commands:%s\n", colorCyan, colorBold, colorReset)
	fmt.Printf("    %s/upload <file...> [--note <note>]%s  Share one or more files in the room\n", colorBold, colorReset)
	fmt.Printf("    %s/files%s                            List all shared files in this room\n", colorBold, colorReset)
	fmt.Printf("    %s/download <file-id> [dir]%s         Download a shared file\n", colorBold, colorReset)
	fmt.Printf("    %s/members%s                          List all members currently in the room\n", colorBold, colorReset)
	if isCreator {
		fmt.Printf("    %s/kick <user-id>%s                   Remove a member from the room (Host only)\n", colorBold, colorReset)
	}
	fmt.Printf("    %s/clear%s                            Clear screen and refresh room header\n", colorBold, colorReset)
	fmt.Printf("    %s/leave%s                            Leave the room and exit\n", colorBold, colorReset)
	fmt.Printf("    %s/help%s                             Show this command reference\n", colorBold, colorReset)
	fmt.Println()
}

func printRoomMessage(m api.RoomMessage) {
	t := time.UnixMilli(m.Timestamp).Format("15:04:05")

	switch m.Type {
	case "system":
		fmt.Printf("  %s[%s]%s %s* %s%s\n", colorDim, t, colorReset, colorYellow, m.Text, colorReset)
	case "file":
		sizeStr := validation.FormatFileSize(m.FileSize)
		fmt.Printf("  %s[%s]%s %s📎 %s shared file:%s %s%s%s (%s) %s[ID: %s]%s\n",
			colorDim, t, colorReset,
			colorMagenta, m.SenderName, colorReset,
			colorBold, m.FileName, colorReset,
			sizeStr,
			colorDim, m.FileID, colorReset,
		)
		if m.Text != "" && m.Text != m.FileName {
			fmt.Printf("         %sNote: \"%s\"%s\n", colorDim, m.Text, colorReset)
		}
	default:
		fmt.Printf("  %s[%s]%s %s%s%s: %s\n",
			colorDim, t, colorReset,
			colorCyan, m.SenderName, colorReset,
			m.Text,
		)
	}
}

func printParticipants(participants []api.RoomParticipant, currentUserID string) {
	fmt.Println()
	fmt.Printf("  %s%s── Room Members (%d) ──%s\n", colorCyan, colorBold, len(participants), colorReset)
	if len(participants) == 0 {
		fmt.Printf("    %sNo active members.%s\n", colorDim, colorReset)
	}
	for _, p := range participants {
		roleBadge := ""
		if p.IsCreator {
			roleBadge = fmt.Sprintf(" %s[Host]👑%s", colorYellow, colorReset)
		}
		youBadge := ""
		if p.ID == currentUserID {
			youBadge = fmt.Sprintf(" %s(You)%s", colorGreen, colorReset)
		}
		fmt.Printf("    • %s%s%s%s%s  (ID: %s%s%s)\n",
			colorBold, p.Name, colorReset,
			roleBadge,
			youBadge,
			colorDim, p.ID, colorReset,
		)
	}
	fmt.Println()
}

func printFiles(files []api.RoomFile) {
	fmt.Println()
	fmt.Printf("  %s%s── Shared Files (%d) ──%s\n", colorCyan, colorBold, len(files), colorReset)
	if len(files) == 0 {
		fmt.Printf("    %sNo files shared yet. Use /upload <file> to share.%s\n", colorDim, colorReset)
	}
	for _, f := range files {
		sizeStr := validation.FormatFileSize(f.Size)
		timeStr := time.UnixMilli(f.UploadedAt).Format("15:04")
		fmt.Printf("    • %s[%s]%s %s%s%s (%s)\n",
			colorCyan, f.ID, colorReset,
			colorBold, f.Name, colorReset,
			sizeStr,
		)
		fmt.Printf("      Uploaded by: %s at %s\n", f.UploaderName, timeStr)
	}
	fmt.Println()
}

func handleInRoomUpload(client *api.Client, roomID int, userID string, args []string) {
	var files []string
	var note string

	for i := 0; i < len(args); i++ {
		if args[i] == "--note" || args[i] == "-n" {
			if i+1 < len(args) {
				note = strings.Join(args[i+1:], " ")
				break
			}
		} else {
			p := cleanPath(args[i])
			if p != "" {
				files = append(files, p)
			}
		}
	}

	if len(files) == 0 {
		fmt.Printf("  %sNo valid files specified to upload.%s\n", colorRed, colorReset)
		return
	}

	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			fmt.Printf("  %sFile not found: %s%s\n", colorRed, f, colorReset)
			return
		}
	}

	fmt.Printf("\n  %sUploading %d file(s) to room %d...%s\n", colorDim, len(files), roomID, colorReset)
	if err := client.UploadRoomFiles(roomID, userID, files, note); err != nil {
		fmt.Printf("  %sUpload failed: %s%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("  %s✓ Uploaded %d file(s) successfully!%s\n\n", colorGreen, len(files), colorReset)
}

func handleInRoomDownload(client *api.Client, roomID int, fileID string, saveDir string) {
	fmt.Printf("\n  %sDownloading file %s from room %d...%s\n", colorDim, fileID, roomID, colorReset)
	savedPath, err := client.DownloadRoomFile(roomID, fileID, saveDir)
	if err != nil {
		fmt.Printf("  %sDownload failed: %s%s\n", colorRed, err, colorReset)
		return
	}

	fi, _ := os.Stat(savedPath)
	var sizeStr string
	if fi != nil {
		sizeStr = validation.FormatFileSize(fi.Size())
	}

	fmt.Printf("  %s✓ Downloaded successfully!%s\n", colorGreen, colorReset)
	fmt.Printf("    Saved to : %s%s%s\n", colorBold, savedPath, colorReset)
	if sizeStr != "" {
		fmt.Printf("    Size     : %s\n", sizeStr)
	}
	fmt.Println()
}
