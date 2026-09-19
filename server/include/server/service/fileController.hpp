#pragma once

#include <atomic>
#include <filesystem>
#include <string>
#include <thread>

#include "server/httplib/httplib.hpp"
#include "server/service/fileSharer.hpp"
#include "server/service/roomManager.hpp"

namespace server::services
{

    class FileController
    {
    public:
        explicit FileController(int port);
        ~FileController();

        // Returns false if the port could not be bound (e.g. another server instance).
        bool start();
        void stop();

    private:
        void registerRoutes();
        void setupHttpLogging();

        // CORS helpers (CORSHandler)
        void applyCorsHeaders(httplib::Response &res) const;
        void handleCorsOrNotFound(const httplib::Request &req, httplib::Response &res) const;

        // Java: UploadHandler
        void handleUpload(const httplib::Request &req, httplib::Response &res);

        // Java: DownloadHandler
        void handleDownload(const httplib::Request &req, httplib::Response &res);

        // Room Handlers
        void handleRoomCreate(const httplib::Request &req, httplib::Response &res);
        void handleRoomJoin(const httplib::Request &req, httplib::Response &res);
        void handleRoomLeave(const httplib::Request &req, httplib::Response &res);
        void handleRoomSync(const httplib::Request &req, httplib::Response &res);
        void handleRoomMessage(const httplib::Request &req, httplib::Response &res);
        void handleRoomUpload(const httplib::Request &req, httplib::Response &res);
        void handleRoomFileDownload(const httplib::Request &req, httplib::Response &res);

        static std::string sanitizeDisplayFilename(const std::string &originalFilename);
        static std::string makeUniqueName(const std::string &originalFilename);

        // Java: path.substring(lastIndexOf('/') + 1) + Integer.parseInt
        static bool parsePortFromPath(const std::string &path, int &outPort);

        service::FileSharer fileSharer_;
        service::RoomManager roomManager_;
        httplib::Server server_;
        std::filesystem::path uploadDir_;
        int port_;

        std::thread serverThread_;
        std::atomic<bool> running_{false};
        std::atomic<bool> listen_ok_{false};
    };

} // namespace server::services
