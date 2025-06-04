# Copyright 2025.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import fcntl
import os
import threading
import time
from typing import Optional, IO
import stat

from logger import setup_logger

logger = setup_logger()


class LockError(Exception):
    pass


class LockManager:
    def __init__(self, lock_path: str, timeout: int = 600):
        self.lock_path = lock_path
        self.timeout = timeout
        self.renew_interval = 300
        self.stop_renew_event = threading.Event()
        self._lock_file: Optional[IO] = None
        self._renew_thread: Optional[threading.Thread] = None
        self._is_locked = False

    def __enter__(self) -> 'LockManager':
        if not self.try_acquire():
            raise LockError(f"Failed to acquire lock: {self.lock_path}")
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        self.release()

    @property
    def is_locked(self) -> bool:
        return self._is_locked

    def try_acquire(self) -> bool:
        if self._is_locked:
            return True
        try:
            lock_dir = os.path.dirname(self.lock_path)
            os.makedirs(lock_dir, exist_ok=True)
            if os.path.exists(self.lock_path):
                mtime = os.path.getmtime(self.lock_path)
                if time.time() - mtime < self.timeout:
                    logger.info(f"Lock file exists and is not expired: {self.lock_path}")
                    return False
            self._lock_file = open(self.lock_path, "w")
            os.chmod(self.lock_path, stat.S_IRUSR | stat.S_IWUSR)
            fcntl.flock(self._lock_file, fcntl.LOCK_EX | fcntl.LOCK_NB)
            self._lock_file.write(f"{os.getpid()}\n")
            self._lock_file.flush()
            os.utime(self.lock_path, None)
            self._is_locked = True
            self._start_renew_thread()
            logger.info(f"Lock acquired: {self.lock_path}")
            return True
        except Exception as e:
            logger.error(f"Error acquiring lock: {e}")
            self._cleanup()
            return False

    def release(self) -> None:
        if not self._is_locked:
            return
        self.stop_renew()
        if self._lock_file:
            try:
                fcntl.flock(self._lock_file, fcntl.LOCK_UN)
                self._lock_file.close()
                if os.path.exists(self.lock_path):
                    os.remove(self.lock_path)
                    logger.info(f"Lock released and file removed: {self.lock_path}")
            except Exception as e:
                logger.error(f"Error releasing lock: {e}")
            finally:
                self._cleanup()

    def _cleanup(self) -> None:
        if self._lock_file:
            try:
                self._lock_file.close()
            except Exception as e:
                logger.error(f"Error while closing lock file: {e}")
        self._lock_file = None
        self._is_locked = False

    def _start_renew_thread(self) -> None:
        self.stop_renew_event.clear()
        self._renew_thread = threading.Thread(
            target=self.renew,
            args=(self.renew_interval,),
            daemon=True
        )
        self._renew_thread.start()

    def renew(self, interval: int = 300) -> None:
        while not self.stop_renew_event.is_set():
            if not self._is_locked:
                break
            if os.path.exists(self.lock_path):
                try:
                    os.utime(self.lock_path, None)
                    logger.debug(f"Lock renewed: {self.lock_path}")
                except IOError as e:
                    logger.error(f"IOError while renewing lock: {e}")
            else:
                logger.warning(f"Lock file does not exist, renew skipped: {self.lock_path}")
            self.stop_renew_event.wait(interval)

    def stop_renew(self) -> None:
        self.stop_renew_event.set()
        if self._renew_thread and self._renew_thread.is_alive():
            self._renew_thread.join(timeout=1.0)
