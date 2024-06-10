import socket
import sys
import threading
import logging
import signal
import time
from functools import partial

from echowarp.logging_config import Logger
from echowarp.start_modes.args_mode import ArgsParser
from echowarp.start_modes.interactive_mode import InteractiveSettings

from echowarp.streamer.audio_client import ClientStreamReceiver
from echowarp.streamer.audio_server import ServerStreamer


def graceful_shutdown(signum, frame, stop_util_event: threading.Event):
    """
    Handles graceful shutdown of the application when a SIGINT signal is received.

    Args:
        signum (int): Signal number.
        frame (frame): Current stack frame.
        stop_util_event (threading.Event): Event to signal the utility to stop.
    """
    logging.info("Gracefully shutting down...")
    stop_util_event.set()


def run_app(stop_util_event: threading.Event(), stop_stream_event: threading.Event()):
    """
        Runs the EchoWarp application in either server or client mode based on provided settings.

        Args:
            stop_util_event (threading.Event): Event to signal the utility to stop.
            stop_stream_event (threading.Event): Event to signal the stream to stop.
        """
    if len(sys.argv) <= 1:
        settings = InteractiveSettings.get_settings_in_interactive_mode()
    else:
        settings = ArgsParser.get_settings_from_cli_args()

    if settings.is_server:
        logging.info("Starting EchoWarp in server mode")
        streamer = ServerStreamer(settings, stop_util_event, stop_stream_event)

        streamer_thread = threading.Thread(target=streamer.encode_audio_and_send_to_client, daemon=True)
        streamer.start_streaming(streamer_thread)
    else:
        logging.info("Starting EchoWarp in client mode")
        receiver = ClientStreamReceiver(settings, stop_util_event, stop_stream_event)

        receiver_thread = threading.Thread(target=receiver.receive_audio_and_decode, daemon=True)
        receiver.start_streaming(receiver_thread)


def main():
    """
        Entry point of the EchoWarp application. Initializes logging, handles signals,
        and runs the application.
        """
    Logger.init_core_logger()

    stop_util_event = threading.Event()
    stop_stream_event = threading.Event()

    signal.signal(signal.SIGINT,
                  partial(graceful_shutdown, stop_util_event=stop_util_event))

    try:
        run_app(stop_util_event, stop_stream_event)
    except (socket.gaierror, OSError) as e:
        logging.error(e)
    except RuntimeError as e:
        logging.error(e)
    except Exception as e:
        logging.critical(f"An error occurred: {e}", exc_info=True)

        stop_stream_event.set()
        stop_util_event.set()
    finally:
        time.sleep(1)
        input("Press Enter to exit...")


if __name__ == "__main__":
    main()
