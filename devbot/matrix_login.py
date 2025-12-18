#!/usr/bin/env python3
"""matrix_login.py - Interactive Matrix login and device verification.

This script handles first-time Matrix login and emoji verification.
Run this once to set up credentials before running matrix_loop.py.

Usage:
    python matrix_login.py

The script will:
1. Prompt for homeserver, username, password (if no credentials exist)
2. Login and store credentials to data/matrix_credentials.json
3. Create E2E key store at data/matrix_store/
4. Wait for and handle emoji verification requests
5. Exit cleanly once verified (or Ctrl-C to exit)
"""

import asyncio
import getpass
import json
import sys
import traceback

from nio import (
    AsyncClient,
    AsyncClientConfig,
    KeyVerificationCancel,
    KeyVerificationEvent,
    KeyVerificationKey,
    KeyVerificationMac,
    KeyVerificationStart,
    LocalProtocolError,
    LoginResponse,
    ToDeviceError,
)
from nio.event_builders import ToDeviceMessage
from nio.events.to_device import UnknownToDeviceEvent

from serve_config import (
    get_matrix_credentials_path,
    get_matrix_store_path,
)


class VerificationCallbacks:
    """Callbacks for handling emoji verification flow."""

    def __init__(self, client: AsyncClient):
        self.client = client

    async def unknown_to_device_callback(self, event: UnknownToDeviceEvent) -> None:
        """Handle unknown to-device events for verification flow."""
        content = event.source.get("content", {})
        transaction_id = content.get("transaction_id")

        if event.type == "m.key.verification.request":
            await self._handle_verification_request(event, content, transaction_id)
        elif event.type == "m.key.verification.done":
            await self._handle_verification_done(event, content, transaction_id)
        else:
            print(f"Received unknown to-device event type: {event.type}")

    async def _handle_verification_request(
        self, event: UnknownToDeviceEvent, content: dict, transaction_id: str
    ) -> None:
        """Handle m.key.verification.request event."""
        try:
            print(f"\nReceived verification request from {event.sender}")

            from_device = content.get("from_device")

            if not transaction_id:
                print("No transaction_id in verification request")
                return

            print(f"  Transaction ID: {transaction_id}")
            print(f"  From device: {from_device}")
            print(f"  Methods: {content.get('methods', [])}")

            # Store the from_device for later use in done handler
            self._pending_verifications = getattr(self, "_pending_verifications", {})
            self._pending_verifications[transaction_id] = from_device

            # Send m.key.verification.ready response
            ready_content = {
                "from_device": self.client.device_id,
                "methods": ["m.sas.v1"],
                "transaction_id": transaction_id,
            }

            message = ToDeviceMessage(
                type="m.key.verification.ready",
                recipient=event.sender,
                recipient_device=from_device,
                content=ready_content,
            )
            resp = await self.client.to_device(message)

            if isinstance(resp, ToDeviceError):
                print(f"Failed to send verification ready: {resp}")
            else:
                print("Sent verification ready response. Waiting for verification to start...")

        except Exception:
            print(traceback.format_exc())

    async def _handle_verification_done(
        self, event: UnknownToDeviceEvent, content: dict, transaction_id: str
    ) -> None:
        """Handle m.key.verification.done event by sending our own done."""
        try:
            print(f"\nReceived verification done from {event.sender}")

            # Get the device we were verifying with
            self._pending_verifications = getattr(self, "_pending_verifications", {})
            from_device = self._pending_verifications.get(transaction_id)

            if not from_device:
                # Try to get from the SAS verification object
                sas = self.client.key_verifications.get(transaction_id)
                if sas:
                    from_device = sas.other_device_id
                else:
                    print(f"Cannot find device for transaction {transaction_id}")
                    return

            # Send our m.key.verification.done response
            done_content = {"transaction_id": transaction_id}

            message = ToDeviceMessage(
                type="m.key.verification.done",
                recipient=event.sender,
                recipient_device=from_device,
                content=done_content,
            )
            resp = await self.client.to_device(message)

            if isinstance(resp, ToDeviceError):
                print(f"Failed to send verification done: {resp}")
            else:
                print("Verification complete! Both sides confirmed.")

            # Clean up
            self._pending_verifications.pop(transaction_id, None)

        except Exception:
            print(traceback.format_exc())

    async def to_device_callback(self, event):
        """Handle to-device events for key verification."""
        try:
            client = self.client

            if isinstance(event, KeyVerificationStart):
                if "emoji" not in event.short_authentication_string:
                    print(
                        "Other device does not support emoji verification "
                        f"{event.short_authentication_string}."
                    )
                    return

                resp = await client.accept_key_verification(event.transaction_id)
                if isinstance(resp, ToDeviceError):
                    print(f"accept_key_verification failed with {resp}")

                sas = client.key_verifications[event.transaction_id]
                todevice_msg = sas.share_key()
                resp = await client.to_device(todevice_msg)
                if isinstance(resp, ToDeviceError):
                    print(f"to_device failed with {resp}")

            elif isinstance(event, KeyVerificationCancel):
                print(
                    f"Verification has been cancelled by {event.sender} "
                    f'for reason "{event.reason}".'
                )

            elif isinstance(event, KeyVerificationKey):
                sas = client.key_verifications[event.transaction_id]
                print(f"\nEmojis for verification: {sas.get_emoji()}")

                yn = input("Do the emojis match? (Y/N/C for Cancel) ")
                if yn.lower() == "y":
                    print("Match! The verification for this device will be accepted.")
                    resp = await client.confirm_short_auth_string(event.transaction_id)
                    if isinstance(resp, ToDeviceError):
                        print(f"confirm_short_auth_string failed with {resp}")
                elif yn.lower() == "n":
                    print("No match! Device will NOT be verified.")
                    resp = await client.cancel_key_verification(
                        event.transaction_id, reject=True
                    )
                    if isinstance(resp, ToDeviceError):
                        print(f"cancel_key_verification failed with {resp}")
                else:
                    print("Cancelled by user!")
                    resp = await client.cancel_key_verification(
                        event.transaction_id, reject=False
                    )
                    if isinstance(resp, ToDeviceError):
                        print(f"cancel_key_verification failed with {resp}")

            elif isinstance(event, KeyVerificationMac):
                sas = client.key_verifications[event.transaction_id]
                try:
                    todevice_msg = sas.get_mac()
                except LocalProtocolError as e:
                    print(
                        f"Cancelled or protocol error: {e}.\n"
                        f"Verification with {event.sender} not concluded. Try again?"
                    )
                else:
                    resp = await client.to_device(todevice_msg)
                    if isinstance(resp, ToDeviceError):
                        print(f"to_device failed with {resp}")
                    print(
                        f"\nsas.we_started_it = {sas.we_started_it}\n"
                        f"sas.sas_accepted = {sas.sas_accepted}\n"
                        f"sas.canceled = {sas.canceled}\n"
                        f"sas.timed_out = {sas.timed_out}\n"
                        f"sas.verified = {sas.verified}\n"
                        f"sas.verified_devices = {sas.verified_devices}\n"
                    )
                    print(
                        "\nEmoji verification was successful!\n"
                        "You can now run matrix_loop.py to start the bot.\n"
                        "Hit Ctrl-C to exit, or initiate another verification."
                    )
            else:
                print(f"Received unexpected event type {type(event)}. Ignoring.")

        except Exception:
            print(traceback.format_exc())


def write_credentials(resp: LoginResponse, homeserver: str) -> None:
    """Write login credentials to disk."""
    creds_path = get_matrix_credentials_path()
    with open(creds_path, "w") as f:
        json.dump(
            {
                "homeserver": homeserver,
                "user_id": resp.user_id,
                "device_id": resp.device_id,
                "access_token": resp.access_token,
            },
            f,
            indent=2,
        )
    print(f"Credentials saved to {creds_path}")


async def login() -> AsyncClient:
    """Handle login with or without stored credentials."""
    client_config = AsyncClientConfig(
        max_limit_exceeded=0,
        max_timeouts=0,
        store_sync_tokens=True,
        encryption_enabled=True,
    )

    creds_path = get_matrix_credentials_path()
    store_path = get_matrix_store_path()

    # Ensure store directory exists
    store_path.mkdir(parents=True, exist_ok=True)

    if not creds_path.exists():
        print(
            "First time setup. No credentials found.\n"
            "Please provide your Matrix account details.\n"
        )

        homeserver = input("Enter your homeserver URL (e.g., matrix.org): ")
        if not (homeserver.startswith("https://") or homeserver.startswith("http://")):
            homeserver = "https://" + homeserver

        user_id = input("Enter your full user ID (e.g., @user:matrix.org): ")
        device_name = input("Choose a name for this device [devbot]: ") or "devbot"
        password = getpass.getpass("Password: ")

        client = AsyncClient(
            homeserver,
            user_id,
            store_path=str(store_path),
            config=client_config,
        )

        resp = await client.login(password=password, device_name=device_name)

        if isinstance(resp, LoginResponse):
            write_credentials(resp, homeserver)
            print(
                "\nLogged in successfully. Credentials stored.\n"
                "On next run, stored credentials will be used."
            )
        else:
            print(f'homeserver = "{homeserver}"; user = "{user_id}"')
            print(f"Failed to log in: {resp}")
            sys.exit(1)

    else:
        with open(creds_path) as f:
            config = json.load(f)

        client = AsyncClient(
            config["homeserver"],
            config["user_id"],
            device_id=config["device_id"],
            store_path=str(store_path),
            config=client_config,
        )

        client.restore_login(
            user_id=config["user_id"],
            device_id=config["device_id"],
            access_token=config["access_token"],
        )
        print(f"Logged in as {config['user_id']} using stored credentials.")

    return client


async def main() -> None:
    """Login and wait for emoji verification."""
    client = await login()

    # Set up verification callbacks
    callbacks = VerificationCallbacks(client)
    client.add_to_device_callback(
        callbacks.to_device_callback, (KeyVerificationEvent,)
    )
    client.add_to_device_callback(
        callbacks.unknown_to_device_callback, (UnknownToDeviceEvent,)
    )

    # Upload encryption keys if needed
    if client.should_upload_keys:
        await client.keys_upload()

    print(
        "\nReady and waiting for emoji verification.\n"
        'Initiate verification from another device by selecting "Verify by Emoji".\n'
        "Press Ctrl-C to exit.\n"
    )

    await client.sync_forever(timeout=30000, full_state=True)


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nExiting.")
        sys.exit(0)
    except Exception:
        print(traceback.format_exc())
        sys.exit(1)
