#include "server/service/roomManager.hpp"
#include "server/utils/uploadUtils.hpp"
#include "server/logging.hpp"

#include <algorithm>
#include <cstdio>
#include <random>

namespace server::service
{

    RoomManager::RoomManager() = default;

    RoomManager::~RoomManager()
    {
        shutdown();
    }

    void RoomManager::shutdown()
    {
        std::lock_guard<std::mutex> lock(roomsMutex_);
        for (auto &[port, room] : rooms_)
        {
            for (auto &[fileId, file] : room.files)
            {
                std::error_code ec;
                std::filesystem::remove(file.diskPath, ec);
            }
            room.files.clear();
            room.participants.clear();
            room.messages.clear();
        }
        rooms_.clear();
    }

    int64_t RoomManager::currentTimestampMs()
    {
        return std::chrono::duration_cast<std::chrono::milliseconds>(
                   std::chrono::system_clock::now().time_since_epoch())
            .count();
    }

    std::string RoomManager::generateId(const std::string &prefix)
    {
        static const char *kHex = "0123456789abcdef";
        thread_local std::mt19937 rng{std::random_device{}()};
        std::uniform_int_distribution<int> dist(0, 15);
        std::string s = prefix;
        for (int i = 0; i < 16; ++i)
        {
            s.push_back(kHex[dist(rng)]);
        }
        return s;
    }

    std::string RoomManager::escapeJson(const std::string &s)
    {
        std::string out;
        out.reserve(s.size() + 8);
        for (char c : s)
        {
            switch (c)
            {
            case '"':
                out += "\\\"";
                break;
            case '\\':
                out += "\\\\";
                break;
            case '\b':
                out += "\\b";
                break;
            case '\f':
                out += "\\f";
                break;
            case '\n':
                out += "\\n";
                break;
            case '\r':
                out += "\\r";
                break;
            case '\t':
                out += "\\t";
                break;
            default:
                if (static_cast<unsigned char>(c) < 0x20)
                {
                    char buf[8];
                    std::snprintf(buf, sizeof(buf), "\\u%04x", static_cast<unsigned char>(c));
                    out += buf;
                }
                else
                {
                    out += c;
                }
                break;
            }
        }
        return out;
    }

    void RoomManager::pruneInactiveParticipants(Room &room)
    {
        const auto now = std::chrono::steady_clock::now();
        constexpr auto kInactivityTimeout = std::chrono::seconds(180);

        auto it = room.participants.begin();
        while (it != room.participants.end())
        {
            if (now - it->lastSeen > kInactivityTimeout)
            {
                RoomMessage sysMsg;
                sysMsg.id = generateId("msg_");
                sysMsg.type = "system";
                sysMsg.senderId = "system";
                sysMsg.senderName = "System";
                sysMsg.text = it->name + " timed out due to inactivity";
                sysMsg.timestampMs = currentTimestampMs();
                room.messages.push_back(sysMsg);

                it = room.participants.erase(it);
            }
            else
            {
                ++it;
            }
        }
    }

    bool RoomManager::createRoom(int requestedPort, int maxParticipants, const std::string &creatorName,
                                 std::string &outUserId, int &outPort, std::string &outError)
    {
        std::lock_guard<std::mutex> lock(roomsMutex_);

        int port = requestedPort;
        if (port == 0)
        {
            // Pick an unused port in dynamic range
            constexpr int kMaxAttempts = 500;
            bool found = false;
            for (int i = 0; i < kMaxAttempts; ++i)
            {
                int candidate = server::utils::generateCode();
                if (rooms_.find(candidate) == rooms_.end())
                {
                    port = candidate;
                    found = true;
                    break;
                }
            }
            if (!found)
            {
                outError = "Unable to allocate a free room port";
                return false;
            }
        }
        else
        {
            if (port < 1 || port > 65535)
            {
                outError = "Invalid port number: must be between 1 and 65535";
                return false;
            }
            if (rooms_.find(port) != rooms_.end())
            {
                outError = "A room with port " + std::to_string(port) + " already exists";
                return false;
            }
        }

        int capacity = maxParticipants;
        if (capacity < 2)
            capacity = 2;
        if (capacity > 100)
            capacity = 100;

        std::string cName = creatorName.empty() ? "Host" : creatorName;
        std::string userId = generateId("usr_");
        int64_t nowMs = currentTimestampMs();
        auto nowSteady = std::chrono::steady_clock::now();

        Room room;
        room.id = port;
        room.maxParticipants = capacity;
        room.creatorId = userId;
        room.createdAt = nowSteady;
        room.lastActivity = nowSteady;

        RoomParticipant creator;
        creator.id = userId;
        creator.name = cName;
        creator.isCreator = true;
        creator.joinedAtMs = nowMs;
        creator.lastSeen = nowSteady;
        room.participants.push_back(creator);

        RoomMessage welcomeMsg;
        welcomeMsg.id = generateId("msg_");
        welcomeMsg.type = "system";
        welcomeMsg.senderId = "system";
        welcomeMsg.senderName = "System";
        welcomeMsg.text = "Room " + std::to_string(port) + " created by " + cName +
                          " (max " + std::to_string(capacity) + " participants)";
        welcomeMsg.timestampMs = nowMs;
        room.messages.push_back(welcomeMsg);

        rooms_[port] = std::move(room);

        outUserId = userId;
        outPort = port;
        return true;
    }

    bool RoomManager::joinRoom(int port, const std::string &userName,
                               std::string &outUserId, int &outMaxParticipants,
                               std::string &outError)
    {
        std::lock_guard<std::mutex> lock(roomsMutex_);

        auto it = rooms_.find(port);
        if (it == rooms_.end())
        {
            outError = "Room " + std::to_string(port) + " does not exist";
            return false;
        }

        Room &room = it->second;
        pruneInactiveParticipants(room);

        if (static_cast<int>(room.participants.size()) >= room.maxParticipants)
        {
            outError = "Room is full (capacity " + std::to_string(room.maxParticipants) + " reached)";
            return false;
        }

        std::string name = userName.empty() ? ("User-" + generateId("").substr(0, 4)) : userName;
        std::string userId = generateId("usr_");
        int64_t nowMs = currentTimestampMs();
        auto nowSteady = std::chrono::steady_clock::now();

        RoomParticipant participant;
        participant.id = userId;
        participant.name = name;
        participant.isCreator = false;
        participant.joinedAtMs = nowMs;
        participant.lastSeen = nowSteady;
        room.participants.push_back(participant);

        room.lastActivity = nowSteady;

        RoomMessage joinMsg;
        joinMsg.id = generateId("msg_");
        joinMsg.type = "system";
        joinMsg.senderId = "system";
        joinMsg.senderName = "System";
        joinMsg.text = name + " joined the room";
        joinMsg.timestampMs = nowMs;
        room.messages.push_back(joinMsg);

        outUserId = userId;
        outMaxParticipants = room.maxParticipants;
        return true;
    }

    bool RoomManager::leaveRoom(int port, const std::string &userId, std::string &outError)
    {
        std::lock_guard<std::mutex> lock(roomsMutex_);

        auto it = rooms_.find(port);
        if (it == rooms_.end())
        {
            outError = "Room not found";
            return false;
        }

        Room &room = it->second;
        std::string leftName;
        auto pIt = std::find_if(room.participants.begin(), room.participants.end(),
                                [&](const RoomParticipant &p)
                                { return p.id == userId; });

        if (pIt != room.participants.end())
        {
            leftName = pIt->name;
            room.participants.erase(pIt);

            RoomMessage leaveMsg;
            leaveMsg.id = generateId("msg_");
            leaveMsg.type = "system";
            leaveMsg.senderId = "system";
            leaveMsg.senderName = "System";
            leaveMsg.text = leftName + " left the room";
            leaveMsg.timestampMs = currentTimestampMs();
            room.messages.push_back(leaveMsg);
            room.lastActivity = std::chrono::steady_clock::now();
        }

        if (room.participants.empty())
        {
            // All members left, cleanup room disk files
            for (auto &[fileId, file] : room.files)
            {
                std::error_code ec;
                std::filesystem::remove(file.diskPath, ec);
            }
            rooms_.erase(it);
        }

        return true;
    }

    bool RoomManager::addMessage(int port, const std::string &userId, const std::string &text,
                                 RoomMessage &outMessage, std::string &outError)
    {
        std::lock_guard<std::mutex> lock(roomsMutex_);

        auto it = rooms_.find(port);
        if (it == rooms_.end())
        {
            outError = "Room not found";
            return false;
        }

        Room &room = it->second;
        auto pIt = std::find_if(room.participants.begin(), room.participants.end(),
                                [&](const RoomParticipant &p)
                                { return p.id == userId; });

        if (pIt == room.participants.end())
        {
            outError = "Participant not found in room. Please rejoin.";
            return false;
        }

        pIt->lastSeen = std::chrono::steady_clock::now();
        room.lastActivity = pIt->lastSeen;

        RoomMessage msg;
        msg.id = generateId("msg_");
        msg.type = "chat";
        msg.senderId = userId;
        msg.senderName = pIt->name;
        msg.text = text;
        msg.timestampMs = currentTimestampMs();

        room.messages.push_back(msg);
        outMessage = msg;
        return true;
    }

    bool RoomManager::addFile(int port, const std::string &userId, const std::string &diskPath,
                             const std::string &originalName, std::size_t sizeBytes,
                             const std::string &optionalNote,
                             RoomFile &outFile, RoomMessage &outMessage, std::string &outError)
    {
        std::lock_guard<std::mutex> lock(roomsMutex_);

        auto it = rooms_.find(port);
        if (it == rooms_.end())
        {
            outError = "Room not found";
            return false;
        }

        Room &room = it->second;
        auto pIt = std::find_if(room.participants.begin(), room.participants.end(),
                                [&](const RoomParticipant &p)
                                { return p.id == userId; });

        if (pIt == room.participants.end())
        {
            outError = "Participant not found in room. Please rejoin.";
            return false;
        }

        pIt->lastSeen = std::chrono::steady_clock::now();
        room.lastActivity = pIt->lastSeen;

        RoomFile rf;
        rf.id = generateId("fil_");
        rf.originalName = originalName;
        rf.diskPath = diskPath;
        rf.sizeBytes = sizeBytes;
        rf.uploaderId = userId;
        rf.uploaderName = pIt->name;
        rf.uploadedAtMs = currentTimestampMs();

        room.files[rf.id] = rf;

        RoomMessage msg;
        msg.id = generateId("msg_");
        msg.type = "file";
        msg.senderId = userId;
        msg.senderName = pIt->name;
        msg.text = optionalNote;
        msg.fileId = rf.id;
        msg.fileName = originalName;
        msg.fileSize = sizeBytes;
        msg.timestampMs = rf.uploadedAtMs;

        room.messages.push_back(msg);

        outFile = rf;
        outMessage = msg;
        return true;
    }

    bool RoomManager::getRoomFile(int port, const std::string &fileId, RoomFile &outFile) const
    {
        std::lock_guard<std::mutex> lock(roomsMutex_);

        auto it = rooms_.find(port);
        if (it == rooms_.end())
            return false;

        const Room &room = it->second;
        auto fIt = room.files.find(fileId);
        if (fIt == room.files.end())
            return false;

        outFile = fIt->second;
        return true;
    }

    bool RoomManager::getRoomSync(int port, const std::string &userId, int64_t sinceMs,
                                  std::string &outJson, std::string &outError)
    {
        std::lock_guard<std::mutex> lock(roomsMutex_);

        auto it = rooms_.find(port);
        if (it == rooms_.end())
        {
            outError = "Room not found";
            return false;
        }

        Room &room = it->second;

        if (!userId.empty())
        {
            for (auto &p : room.participants)
            {
                if (p.id == userId)
                {
                    p.lastSeen = std::chrono::steady_clock::now();
                    break;
                }
            }
        }

        pruneInactiveParticipants(room);

        // Build JSON response
        std::string json = "{";
        json += "\"roomId\":" + std::to_string(room.id) + ",";
        json += "\"maxParticipants\":" + std::to_string(room.maxParticipants) + ",";

        // Participants
        json += "\"participants\":[";
        for (std::size_t i = 0; i < room.participants.size(); ++i)
        {
            const auto &p = room.participants[i];
            if (i > 0)
                json += ",";
            json += "{";
            json += "\"id\":\"" + escapeJson(p.id) + "\",";
            json += "\"name\":\"" + escapeJson(p.name) + "\",";
            json += "\"isCreator\":" + std::string(p.isCreator ? "true" : "false") + ",";
            json += "\"joinedAt\":" + std::to_string(p.joinedAtMs);
            json += "}";
        }
        json += "],";

        // Messages
        json += "\"messages\":[";
        bool firstMsg = true;
        for (const auto &m : room.messages)
        {
            if (m.timestampMs >= sinceMs)
            {
                if (!firstMsg)
                    json += ",";
                firstMsg = false;

                json += "{";
                json += "\"id\":\"" + escapeJson(m.id) + "\",";
                json += "\"type\":\"" + escapeJson(m.type) + "\",";
                json += "\"senderId\":\"" + escapeJson(m.senderId) + "\",";
                json += "\"senderName\":\"" + escapeJson(m.senderName) + "\",";
                json += "\"text\":\"" + escapeJson(m.text) + "\",";
                json += "\"fileId\":\"" + escapeJson(m.fileId) + "\",";
                json += "\"fileName\":\"" + escapeJson(m.fileName) + "\",";
                json += "\"fileSize\":" + std::to_string(m.fileSize) + ",";
                json += "\"timestamp\":" + std::to_string(m.timestampMs);
                json += "}";
            }
        }
        json += "],";

        // Files
        json += "\"files\":[";
        std::size_t fileIdx = 0;
        for (const auto &[fid, f] : room.files)
        {
            if (fileIdx > 0)
                json += ",";
            json += "{";
            json += "\"id\":\"" + escapeJson(f.id) + "\",";
            json += "\"name\":\"" + escapeJson(f.originalName) + "\",";
            json += "\"size\":" + std::to_string(f.sizeBytes) + ",";
            json += "\"uploaderId\":\"" + escapeJson(f.uploaderId) + "\",";
            json += "\"uploaderName\":\"" + escapeJson(f.uploaderName) + "\",";
            json += "\"uploadedAt\":" + std::to_string(f.uploadedAtMs);
            json += "}";
            fileIdx++;
        }
        json += "]}";

        outJson = std::move(json);
        return true;
    }

    void RoomManager::heartbeat(int port, const std::string &userId)
    {
        std::lock_guard<std::mutex> lock(roomsMutex_);
        auto it = rooms_.find(port);
        if (it == rooms_.end())
            return;

        for (auto &p : it->second.participants)
        {
            if (p.id == userId)
            {
                p.lastSeen = std::chrono::steady_clock::now();
                break;
            }
        }
    }

    bool RoomManager::roomExists(int port) const
    {
        std::lock_guard<std::mutex> lock(roomsMutex_);
        return rooms_.find(port) != rooms_.end();
    }

} // namespace server::service
