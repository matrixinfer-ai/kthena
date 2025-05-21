import fcntl
import os
import threading

from logger import setup_logger

logger = setup_logger()


class LockManager:
    def __init__(self, lock_path: str, timeout: int = 600):
        self.lock_path = lock_path
        self.timeout = timeout
        self.lock_acquired = False
        self.renew_interval = 300
        self.stop_renew_event = threading.Event()

    def try_acquire(self) -> bool:
        try:
            lock_dir = os.path.dirname(self.lock_path)
            os.makedirs(lock_dir, exist_ok=True)
            with open(self.lock_path, "w") as f:
                fcntl.flock(f, fcntl.LOCK_EX | fcntl.LOCK_NB)
                f.write(str(os.getpid()))
                self.lock_acquired = True
                return True
        except (IOError, BlockingIOError):
            return False
        except Exception as e:
            logger.error(f"Error acquiring lock: {e}")
            return False

    def release(self):
        if self.lock_acquired:
            try:
                if os.path.exists(self.lock_path):
                    os.remove(self.lock_path)
                self.lock_acquired = False
            except Exception as e:
                logger.error(f"Error releasing lock: {e}")

    def renew(self, interval: int = 300):
        while not self.stop_renew_event.is_set():
            if self.lock_acquired:
                try:
                    if os.path.exists(self.lock_path):
                        os.utime(self.lock_path)
                        logger.info(f"Renewed lock at {self.lock_path}")
                    else:
                        logger.warning(f"Lock file {self.lock_path} does not exist. Renew skipped.")
                except Exception as e:
                    logger.error(f"Error renewing lock: {e}")
            self.stop_renew_event.wait(interval)

    def stop_renew(self):
        self.stop_renew_event.set()
