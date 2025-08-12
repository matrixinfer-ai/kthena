# Copyright MatrixInfer-AI Authors.
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

import os
import time
import unittest
from unittest.mock import patch

from matrixinfer.downloader.lock import LockManager, LockError
from matrixinfer.tests.test_utils import create_temp_dir, cleanup_temp_dir


class TestLockManager(unittest.TestCase):
    def setUp(self):
        self.temp_dir = create_temp_dir()
        self.lock_path = os.path.join(self.temp_dir, "test.lock")
        self.lock_manager = LockManager(self.lock_path)

    def tearDown(self):
        if hasattr(self, "lock_manager") and self.lock_manager.is_locked:
            self.lock_manager.release()
        cleanup_temp_dir(self.temp_dir)

    def _acquire_lock(self, manager=None):
        manager = manager or self.lock_manager
        result = manager.try_acquire()
        self.assertTrue(result, "Failed to acquire lock")
        return result

    def _assert_lock_file_exists(self):
        self.assertTrue(os.path.exists(self.lock_path), "Lock file should exist")

    def _assert_lock_file_removed(self):
        self.assertFalse(os.path.exists(self.lock_path), "Lock file should be removed")

    def test_lock_manager_acquire_release(self):
        self._acquire_lock()
        self._assert_lock_file_exists()
        self.lock_manager.release()
        self._assert_lock_file_removed()

    def test_lock_manager_context_manager(self):
        with LockManager(self.lock_path) as lock_manager:
            self._assert_lock_file_exists()
            self.assertTrue(lock_manager.is_locked, "Lock should be marked as locked")
        self._assert_lock_file_removed()

    def test_lock_manager_renew(self):
        self._acquire_lock()
        time.sleep(0.5)
        self._assert_lock_file_exists()
        self.lock_manager.release()
        self._assert_lock_file_removed()

    def test_lock_manager_acquire_failure(self):
        lock_manager1 = LockManager(self.lock_path)
        lock_manager2 = LockManager(self.lock_path)

        self._acquire_lock(lock_manager1)
        self.assertFalse(
            lock_manager2.try_acquire(),
            "Second lock manager should fail to acquire the lock",
        )
        lock_manager1.release()

    def test_context_manager_failure(self):
        self._acquire_lock()
        with self.assertRaises(LockError):
            with LockManager(self.lock_path):
                pass
        self.lock_manager.release()

    def test_is_locked_property(self):
        self.assertFalse(
            self.lock_manager.is_locked, "is_locked should be False initially"
        )
        self._acquire_lock()
        self.assertTrue(
            self.lock_manager.is_locked, "is_locked should be True after acquisition"
        )
        self.lock_manager.release()
        self.assertFalse(
            self.lock_manager.is_locked, "is_locked should be False after release"
        )

    def test_renew_thread_starts_automatically(self):
        with patch.object(
            self.lock_manager, "_start_renew_thread"
        ) as mock_start_thread:
            self.lock_manager.try_acquire()
            mock_start_thread.assert_called_once()
            self.lock_manager.release()

    def test_renew_thread_stops_on_release(self):
        self._acquire_lock()
        self.assertIsNotNone(
            self.lock_manager._renew_thread, "Renew thread should be created"
        )
        self.assertTrue(
            self.lock_manager._renew_thread.is_alive(), "Renew thread should be running"
        )

        self.lock_manager.release()
        self.assertFalse(
            (
                self.lock_manager._renew_thread.is_alive()
                if self.lock_manager._renew_thread
                else True
            ),
            "Renew thread should be stopped",
        )

    @patch("matrixinfer.downloader.lock.logger.error")
    def test_lock_manager_acquire_exception_handling(self, mock_logger_error):
        with patch("os.makedirs", side_effect=OSError("Mocked error")):
            self.assertFalse(
                self.lock_manager.try_acquire(),
                "Acquire should fail due to mocked OSError",
            )
            mock_logger_error.assert_called_with("Error acquiring lock: Mocked error")

    @patch("matrixinfer.downloader.lock.logger.error")
    def test_lock_manager_release_exception_handling(self, mock_logger_error):
        self._acquire_lock()
        with patch("os.remove", side_effect=OSError("Mocked error")):
            self.lock_manager.release()
            mock_logger_error.assert_called_with("Error releasing lock: Mocked error")

    def test_lock_manager_renew_lock_file_missing(self):
        self._acquire_lock()
        os.remove(self.lock_path)
        time.sleep(0.5)
        self.lock_manager.release()

    def test_lock_manager_stop_renew(self):
        self._acquire_lock()
        self.assertTrue(
            self.lock_manager._renew_thread.is_alive(), "Renew thread should be running"
        )
        self.lock_manager.stop_renew()
        max_wait = 0.2
        start_time = time.time()
        while time.time() - start_time < max_wait and (
            self.lock_manager._renew_thread
            and self.lock_manager._renew_thread.is_alive()
        ):
            time.sleep(0.02)
        self.assertTrue(
            not self.lock_manager._renew_thread
            or not self.lock_manager._renew_thread.is_alive(),
            "Renew thread should be stopped or None",
        )
        self.lock_manager.release()

    def test_multiple_locks_management(self):
        lock_paths = [os.path.join(self.temp_dir, f"lock_{i}.lock") for i in range(3)]
        lock_managers = [LockManager(path) for path in lock_paths]

        for manager in lock_managers:
            self.assertTrue(manager.try_acquire(), "Failed to acquire multiple locks")

        for path in lock_paths:
            self.assertTrue(os.path.exists(path), "Lock file should exist")

        for manager in lock_managers:
            manager.release()

        for path in lock_paths:
            self.assertFalse(
                os.path.exists(path), "Lock file should be removed after release"
            )


if __name__ == "__main__":
    unittest.main()
