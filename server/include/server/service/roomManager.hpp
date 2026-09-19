#pragma once

#include <chrono>
#include <cstdint>
#include <filesystem>
#include <memory>
#include <mutex>
#include <string>
#include <unordered_map>
#include <vector>

namespace server::service
{

    struct RoomParticipant
    {
        std::string id;
        std::string name;
        bool isCreator{false};
        int64_t joinedAtMs{0};
        std::chrono::steady_clock::time_point lastSeen{};
    };

    struct RoomFile
    {
        std::string id;
        std::string originalName;
        std::string diskPath;
        std::size_t sizeBytes{0};
        std::string uploaderId;
        std::string uploaderName;
        int64_t uploadedAtMs{0};
    };

    struct RoomMessage
    {
        std::string id;
        std::string type; // "system", "chat", "file"
        std::string senderId;
        std::string senderName;
        std::string text;
        std::string fileId;
        std::string fileName;
        std::size_t fileSize{0};
        int64_t timestampMs{0};
    };

    struct Room
    {
        int id{0}; // Room ID / Port
        int maxParticipants{5};
        std::string creatorId;
        std::chrono::steady_clock::time_point createdAt{};
        std::chrono::steady_clock::time_point lastActivity{};

        std::vector<RoomParticipant> participants;
        std::vector<RoomMessage> messages;
        std::unordered_map<std::string, RoomFile> files;
    };

    class RoomManager
    {
    public:
        RoomManager();
        ~RoomManager();

        RoomManager(const RoomManager &) = delete;
        RoomManager &operator=(const RoomManager &) = delete;

        void shutdown();

        // Room management
        bool createRoom(int requestedPort, int maxParticipants, const std::string &creatorName,
                        std::string &outUserId, int &outPort, std::string &outError);

        bool joinRoom(int port, const std::string &userName,
                      std::string &outUserId, int &outMaxParticipants,
                      std::string &outError);

        bool leaveRoom(int port, const std::string &userId, std::string &outError);

        bool addMessage(int port, const std::string &userId, const std::string &text,
                        RoomMessage &outMessage, std::string &outError);

        bool addFile(int port, const std::string &userId, const std::string &diskPath,
                     const std::string &originalName, std::size_t sizeBytes,
                     const std::string &optionalNote,
                     RoomFile &outFile, RoomMessage &outMessage, std::string &outError);

        bool getRoomFile(int port, const std::string &fileId, RoomFile &outFile) const;

        bool getRoomSync(int port, const std::string &userId, int64_t sinceMs,
                         std::string &outJson, std::string &outError);

        void heartbeat(int port, const std::string &userId);

        bool roomExists(int port) const;

    private:
        mutable std::mutex roomsMutex_;
        std::unordered_map<int, Room> rooms_;

        static int64_t currentTimestampMs();
        static std::string generateId(const std::string &prefix);
        static std::string escapeJson(const std::string &s);

        void pruneInactiveParticipants(Room &room);
    };

} // namespace server::service
