import os
import time
import threading
import unittest
from unittest.mock import patch

from lock import LockManager
from tests.test_utils import create_temp_dir, cleanup_temp_dir


class TestLockManager(unittest.TestCase):
    def setUp(self):
        self.temp_dir = create_temp_dir()
        self.lock_path = os.path.join(self.temp_dir, "test.lock")

    def tearDown(self):
        cleanup_temp_dir(self.temp_dir)

    def test_lock_manager_acquire_release(self):
        lock_manager = LockManager(self.lock_path)
        self.assertTrue(lock_manager.try_acquire(), "Failed to acquire lock")
        self.assertTrue(os.path.exists(self.lock_path), "Lock file should exist after acquisition")
        lock_manager.release()
        self.assertFalse(os.path.exists(self.lock_path), "Lock file should be removed after release")

    def test_lock_manager_renew(self):
        lock_manager = LockManager(self.lock_path)
        self.assertTrue(lock_manager.try_acquire(), "Failed to acquire lock")
        renew_thread = threading.Thread(target=lock_manager.renew, args=(1,), daemon=True)
        renew_thread.start()
        self.assertTrue(os.path.exists(self.lock_path), "Lock file should exist during renewal")
        lock_manager.stop_renew()
        renew_thread.join(timeout=2)
        self.assertTrue(os.path.exists(self.lock_path), "Lock file should still exist after stopping renewal")
        lock_manager.release()

    def test_lock_manager_acquire_failure(self):
        lock_manager1 = LockManager(self.lock_path)
        lock_manager2 = LockManager(self.lock_path)
        self.assertTrue(lock_manager1.try_acquire(), "First lock manager should acquire the lock")
        self.assertFalse(lock_manager2.try_acquire(), "Second lock manager should fail to acquire the lock")
        lock_manager1.release()

    @patch("lock.logger.error")
    def test_lock_manager_acquire_exception_handling(self, mock_logger_error):
        with patch("os.makedirs", side_effect=OSError("Mocked error")):
            lock_manager = LockManager(self.lock_path)
            self.assertFalse(lock_manager.try_acquire(), "Acquire should fail due to mocked OSError")
            mock_logger_error.assert_called_once_with("Error acquiring lock: Mocked error")

    @patch("lock.logger.error")
    def test_lock_manager_release_exception_handling(self, mock_logger_error):
        lock_manager = LockManager(self.lock_path)
        lock_manager.try_acquire()
        with patch("os.remove", side_effect=OSError("Mocked error")):
            lock_manager.release()
            mock_logger_error.assert_called_once_with("Release error: Mocked error")

    def test_lock_manager_renew_lock_file_missing(self):
        lock_manager = LockManager(self.lock_path)
        self.assertTrue(lock_manager.try_acquire(), "Failed to acquire lock")
        renew_thread = threading.Thread(target=lock_manager.renew, args=(1,), daemon=True)
        renew_thread.start()

        os.remove(self.lock_path)
        time.sleep(2)

        lock_manager.stop_renew()
        renew_thread.join(timeout=2)
        self.assertFalse(os.path.exists(self.lock_path), "Lock file should remain missing after renewal failure")
        lock_manager.release()

    def test_lock_manager_stop_renew(self):
        lock_manager = LockManager(self.lock_path)
        self.assertTrue(lock_manager.try_acquire(), "Failed to acquire lock")
        renew_thread = threading.Thread(target=lock_manager.renew, args=(1,), daemon=True)
        renew_thread.start()

        lock_manager.stop_renew()
        renew_thread.join(timeout=2)
        self.assertFalse(renew_thread.is_alive(), "Renew thread should terminate after stop_renew is called")
        lock_manager.release()


if __name__ == "__main__":
    unittest.main()
