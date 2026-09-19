#include "server/service/roomManager.hpp"
#include <cassert>
#include <iostream>
#include <fstream>

int main()
{
    std::cout << "[TEST] Starting RoomManager tests..." << std::endl;

    server::service::RoomManager manager;

    // 1. Create Room with maxParticipants = 2
    std::string creatorId;
    int port = 0;
    std::string err;
    bool ok = manager.createRoom(55555, 2, "Alice", creatorId, port, err);
    assert(ok);
    assert(port == 55555);
    assert(!creatorId.empty());
    std::cout << "  ✓ CreateRoom succeeded (port 55555, creator: " << creatorId << ")" << std::endl;

    // 2. Duplicate port rejected
    std::string dupUserId;
    int dupPort = 0;
    ok = manager.createRoom(55555, 5, "Bob", dupUserId, dupPort, err);
    assert(!ok);
    std::cout << "  ✓ Duplicate port rejected properly: " << err << std::endl;

    // 3. User B joins room 55555
    std::string userBId;
    int maxPart = 0;
    ok = manager.joinRoom(55555, "Bob", userBId, maxPart, err);
    assert(ok);
    assert(maxPart == 2);
    assert(!userBId.empty());
    std::cout << "  ✓ User B (Bob) joined room (id: " << userBId << ")" << std::endl;

    // 4. Capacity limit enforced: User C attempts to join (capacity 2 is full: Alice + Bob)
    std::string userCId;
    ok = manager.joinRoom(55555, "Charlie", userCId, maxPart, err);
    assert(!ok);
    std::cout << "  ✓ Capacity limit enforced correctly: " << err << std::endl;

    // 5. Send Chat Message
    server::service::RoomMessage msg;
    ok = manager.addMessage(55555, userBId, "Hello Alice!", msg, err);
    assert(ok);
    assert(msg.senderName == "Bob");
    assert(msg.text == "Hello Alice!");
    assert(msg.type == "chat");
    std::cout << "  ✓ Chat message added: " << msg.senderName << ": " << msg.text << std::endl;

    // 6. Share File in Room
    std::string tempFile = "/tmp/test_room_file.txt";
    {
        std::ofstream ofs(tempFile);
        ofs << "Hello Transfera Room!";
    }
    server::service::RoomFile rf;
    server::service::RoomMessage fmsg;
    ok = manager.addFile(55555, creatorId, tempFile, "test.txt", 21, "Here is the file", rf, fmsg, err);
    assert(ok);
    assert(rf.originalName == "test.txt");
    assert(rf.sizeBytes == 21);
    assert(fmsg.type == "file");
    std::cout << "  ✓ File shared in room: " << rf.originalName << " (id: " << rf.id << ")" << std::endl;

    // 7. Get File
    server::service::RoomFile retrieved;
    ok = manager.getRoomFile(55555, rf.id, retrieved);
    assert(ok);
    assert(retrieved.sizeBytes == 21);
    assert(retrieved.originalName == "test.txt");
    std::cout << "  ✓ Room file retrieved successfully" << std::endl;

    // 8. Sync Room
    std::string syncJson;
    ok = manager.getRoomSync(55555, creatorId, 0, syncJson, err);
    assert(ok);
    assert(syncJson.find("\"roomId\":55555") != std::string::npos);
    assert(syncJson.find("Alice") != std::string::npos);
    assert(syncJson.find("Bob") != std::string::npos);
    assert(syncJson.find("Hello Alice!") != std::string::npos);
    assert(syncJson.find("test.txt") != std::string::npos);
    std::cout << "  ✓ Room sync JSON valid: " << syncJson.substr(0, 100) << "..." << std::endl;

    // 9. Bob leaves room -> capacity frees up
    ok = manager.leaveRoom(55555, userBId, err);
    assert(ok);
    std::cout << "  ✓ Bob left room" << std::endl;

    // 10. Charlie can now join since Bob left
    ok = manager.joinRoom(55555, "Charlie", userCId, maxPart, err);
    assert(ok);
    std::cout << "  ✓ Charlie successfully joined after Bob left!" << std::endl;

    // 11. Alice and Charlie leave -> room destroyed and temp file removed
    manager.leaveRoom(55555, creatorId, err);
    manager.leaveRoom(55555, userCId, err);
    assert(!manager.roomExists(55555));
    std::cout << "  ✓ Room destroyed when empty, cleaned up" << std::endl;

    std::cout << "\n[ALL ROOM TESTS PASSED SUCCESSFULLY!]" << std::endl;
    return 0;
}
