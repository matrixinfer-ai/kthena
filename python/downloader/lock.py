# lock.py

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
        self._lock_file = None

    def try_acquire(self) -> bool:
        try:
            lock_dir = os.path.dirname(self.lock_path)
            os.makedirs(lock_dir, exist_ok=True)
            self._lock_file = open(self.lock_path, "w")
            fcntl.flock(self._lock_file, fcntl.LOCK_EX | fcntl.LOCK_NB)
            self._lock_file.write(str(os.getpid()))
            self._lock_file.flush()
            self.lock_acquired = True
            return True
        except Exception as e:
            logger.error(f"Error acquiring lock: {e}")
            if self._lock_file:
                self._lock_file.close()
            return False

    def release(self):
        if self.lock_acquired and self._lock_file:
            try:
                fcntl.flock(self._lock_file, fcntl.LOCK_UN)
                self._lock_file.close()
                if os.path.exists(self.lock_path):
                    os.remove(self.lock_path)
            except Exception as e:
                logger.error(f"Release error: {e}")
            finally:
                self.lock_acquired = False
                self._lock_file = None

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
