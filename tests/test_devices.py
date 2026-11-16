import unittest
from lab.devices import choose_device


class DeviceSelectionTests(unittest.TestCase):
    def test_chooses_only_authorized_device(self):
        self.assertEqual(choose_device("List of devices attached\ndevice_one\tdevice\ndevice_two\toffline\n"),"device_one")
    def test_rejects_missing_authorization(self):
        with self.assertRaises(RuntimeError): choose_device("List of devices attached\ndevice_one\tunauthorized\n")
    def test_multiple_devices_require_explicit_selection(self):
        with self.assertRaises(RuntimeError): choose_device("device_one device\ndevice_two device\n")
