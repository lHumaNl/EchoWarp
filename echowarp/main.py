import sys
import threading
import logging
import signal
from functools import partial

from echowarp.start_modes import args_mode
from echowarp.logging_config import setup_logging
from echowarp.start_modes.interactive_mode import InteractiveSettings

from echowarp.streamer.audio_client import ClientStreamReceiver
from echowarp.streamer.audio_server import ServerStreamer


def graceful_shutdown(signum, frame, stop_util_event: threading.Event, stop_stream_event: threading.Event):
    """
    Handles graceful shutdown of the application when a SIGINT signal is received.

    Args:
        signum (int): Signal number.
        frame (frame): Current stack frame.
        stop_util_event (threading.Event): Event to signal the utility to stop.
        stop_stream_event (threading.Event): Event to signal the streaming to stop.
    """
    logging.info("Gracefully shutting down...")
    stop_stream_event.set()
    stop_util_event.set()


def main():
    setup_logging()

    stop_util_event = threading.Event()
    stop_stream_event = threading.Event()

    signal.signal(signal.SIGINT,
                  partial(graceful_shutdown, stop_util_event=stop_util_event, stop_stream_event=stop_stream_event))

    if len(sys.argv) == 1:
        settings = InteractiveSettings.get_settings_interactive()
    else:
        settings = args_mode.get_settings_by_args()

    if settings.is_server:
        logging.info("Start EchoWarp in server mode")
        streamer = ServerStreamer(settings.udp_port, settings.heartbeat_attempt, stop_util_event, stop_stream_event,
                                  settings.crypto_manager, settings.audio_device, settings.executor)

        streamer_thread = threading.Thread(target=streamer.encode_audio_and_send_to_client, daemon=True)
        streamer.start_heartbeat(streamer_thread)
    else:
        logging.info("Start EchoWarp in client mode")
        receiver = ClientStreamReceiver(settings.server_addr, settings.udp_port, stop_util_event, stop_stream_event,
                                        settings.crypto_manager, settings.audio_device, settings.executor)

        receiver_thread = threading.Thread(target=receiver.receive_audio_and_decode, daemon=True)
        receiver.start_heartbeat(receiver_thread)


if __name__ == "__main__":
    main()
