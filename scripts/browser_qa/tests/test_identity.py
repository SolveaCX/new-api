import unittest

from scripts.browser_qa.flatkey_browser_qa.identity import derive_identity


class IdentityTests(unittest.TestCase):
    def test_identity_is_stable_without_exposing_seed(self):
        first = derive_identity(b"seed-with-32-bytes-minimum-value", "123456789")
        second = derive_identity(b"seed-with-32-bytes-minimum-value", "123456789")

        self.assertEqual(first, second)
        self.assertRegex(first.username, r"^qa[0-9]{8}[a-z0-9]{8}$")
        self.assertRegex(first.email_tag, r"^flatkey-qa-123456789-[a-z0-9]{10}$")
        self.assertEqual(len(first.password), 30)
        self.assertEqual(first.key_name, "cloud-qa-123456789")
        self.assertNotIn("seed", repr(first))

    def test_identity_rejects_short_seed_and_non_decimal_run_id(self):
        with self.assertRaises(ValueError):
            derive_identity(b"short", "123456789")
        with self.assertRaises(ValueError):
            derive_identity(b"seed-with-32-bytes-minimum-value", "run-123")

    def test_password_contains_required_character_classes(self):
        identity = derive_identity(b"seed-with-32-bytes-minimum-value", "987654321")

        self.assertRegex(identity.password, r"[A-Z]")
        self.assertRegex(identity.password, r"[a-z]")
        self.assertRegex(identity.password, r"[0-9]")
        self.assertRegex(identity.password, r"[^A-Za-z0-9]")


if __name__ == "__main__":
    unittest.main()
