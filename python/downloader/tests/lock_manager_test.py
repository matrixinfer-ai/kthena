import os
import time
import unittest
from unittest.mock import patch, MagicMock

from lock import LockManager, LockError
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

    def test_lock_manager_context_manager(self):
        with LockManager(self.lock_path) as lock_manager:
            self.assertTrue(os.path.exists(self.lock_path), "Lock file should exist within context")
            self.assertTrue(lock_manager.is_locked, "Lock should be marked as locked")
        self.assertFalse(os.path.exists(self.lock_path), "Lock file should be removed after context exit")

    def test_lock_manager_renew(self):
        lock_manager = LockManager(self.lock_path)
        self.assertTrue(lock_manager.try_acquire(), "Failed to acquire lock")
        time.sleep(2)
        self.assertTrue(os.path.exists(self.lock_path), "Lock file should exist during renewal")
        lock_manager.release()
        self.assertFalse(os.path.exists(self.lock_path), "Lock file should be removed after release")

    def test_lock_manager_acquire_failure(self):
        lock_manager1 = LockManager(self.lock_path)
        lock_manager2 = LockManager(self.lock_path)
        self.assertTrue(lock_manager1.try_acquire(), "First lock manager should acquire the lock")
        self.assertFalse(lock_manager2.try_acquire(), "Second lock manager should fail to acquire the lock")
        lock_manager1.release()

    def test_context_manager_failure(self):
        lock_manager1 = LockManager(self.lock_path)
        self.assertTrue(lock_manager1.try_acquire(), "First lock manager should acquire the lock")

        with self.assertRaises(LockError):
            with LockManager(self.lock_path):
                pass

        lock_manager1.release()

    def test_is_locked_property(self):
        lock_manager = LockManager(self.lock_path)
        self.assertFalse(lock_manager.is_locked, "is_locked should be False initially")

        lock_manager.try_acquire()
        self.assertTrue(lock_manager.is_locked, "is_locked should be True after acquisition")

        lock_manager.release()
        self.assertFalse(lock_manager.is_locked, "is_locked should be False after release")

    def test_renew_thread_starts_automatically(self):
        lock_manager = LockManager(self.lock_path)
        with patch.object(lock_manager, '_start_renew_thread') as mock_start_thread:
            lock_manager.try_acquire()
            mock_start_thread.assert_called_once()

    def test_renew_thread_stops_on_release(self):
        lock_manager = LockManager(self.lock_path)
        lock_manager.try_acquire()
        self.assertIsNotNone(lock_manager._renew_thread, "Renew thread should be created")
        self.assertTrue(lock_manager._renew_thread.is_alive(), "Renew thread should be running")

        lock_manager.release()
        self.assertFalse(lock_manager._renew_thread.is_alive() if lock_manager._renew_thread else True,
                         "Renew thread should be stopped")

    @patch("lock.logger.error")
    def test_lock_manager_acquire_exception_handling(self, mock_logger_error):
        with patch("os.makedirs", side_effect=OSError("Mocked error")):
            lock_manager = LockManager(self.lock_path)
            self.assertFalse(lock_manager.try_acquire(), "Acquire should fail due to mocked OSError")
            mock_logger_error.assert_called_with("Error acquiring lock: Mocked error")

    @patch("lock.logger.error")
    def test_lock_manager_release_exception_handling(self, mock_logger_error):
        lock_manager = LockManager(self.lock_path)
        lock_manager.try_acquire()
        with patch("os.remove", side_effect=OSError("Mocked error")):
            lock_manager.release()
            mock_logger_error.assert_called_with("Error releasing lock: Mocked error")

    def test_lock_manager_renew_lock_file_missing(self):
        lock_manager = LockManager(self.lock_path)
        self.assertTrue(lock_manager.try_acquire(), "Failed to acquire lock")

        os.remove(self.lock_path)
        time.sleep(2)

        lock_manager.release()

    def test_lock_manager_stop_renew(self):
        lock_manager = LockManager(self.lock_path)
        self.assertTrue(lock_manager.try_acquire(), "Failed to acquire lock")
        self.assertTrue(lock_manager._renew_thread.is_alive(), "Renew thread should be running")

        lock_manager.stop_renew()
        max_wait = 0.5
        start_time = time.time()
        while time.time() - start_time < max_wait and (
                lock_manager._renew_thread and lock_manager._renew_thread.is_alive()):
            time.sleep(0.05)

        self.assertTrue(not lock_manager._renew_thread or not lock_manager._renew_thread.is_alive(),
                        "Renew thread should be stopped or None")
        lock_manager.release()


if __name__ == "__main__":
    unittest.main()
