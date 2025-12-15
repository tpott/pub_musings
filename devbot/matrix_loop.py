import asyncio
from getpass import getpass

from nio import AsyncClient, MatrixRoom, RoomMessageText


async def message_callback(room: MatrixRoom, event: RoomMessageText) -> None:
    print(
        f"Message received in room {room.display_name}\n"
        f"{room.user_name(event.sender)} | {event.body}"
    )


async def main() -> None:
    username = input("username: ")
    password = getpass("password: ")
    client = AsyncClient("https://matrix.org", username)
    client.add_event_callback(message_callback, RoomMessageText)
    print(await client.login(password))
    # await client.join("!zmFHpVupuGWpHKxuZY:matrix.org") # TODO how to enumerate rooms?
    await client.room_send(
        "!zmFHpVupuGWpHKxuZY:matrix.org",  # TODO how to enumerate rooms?
        message_type="m.room.message",
        content={"msgtype": "m.text", "body": "Hello world!"},
    )
    await client.sync_forever(timeout=30000)  # milliseconds
    print("see ya")


if __name__ == "__main__":
    asyncio.run(main())
