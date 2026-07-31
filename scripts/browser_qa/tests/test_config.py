import base64
import unittest

from scripts.browser_qa.flatkey_browser_qa.config import load_config


def valid_env():
    return {
        "FLATKEY_QA_RUN_ID": "123456789",
        "FLATKEY_QA_IDENTITY_SEED_B64": base64.b64encode(b"seed-with-32-bytes-minimum-value").decode("ascii"),
        "FLATKEY_QA_GMAIL_BASE": "owner@gmail.com",
        "FLATKEY_QA_WEBSITE_ORIGIN": "https://staging-website.flatkey.ai",
        "FLATKEY_QA_CONSOLE_ORIGIN": "https://staging-console.flatkey.ai",
        "FLATKEY_QA_DOCS_ORIGIN": "https://docs.flatkey.ai",
    }


class ConfigTests(unittest.TestCase):
    def test_load_config_accepts_strict_values_without_repr_leaks(self):
        cfg = load_config(valid_env())

        self.assertEqual(cfg.run_id, "123456789")
        self.assertNotIn("seed", repr(cfg))
        self.assertNotIn("owner", repr(cfg))
        self.assertNotIn("gmail", repr(cfg).lower())

    def test_load_config_rejects_missing_and_unknown_flatkey_env(self):
        missing = valid_env()
        del missing["FLATKEY_QA_RUN_ID"]
        with self.assertRaises(ValueError):
            load_config(missing)

        unknown = valid_env()
        unknown["FLATKEY_QA_EXTRA"] = "nope"
        with self.assertRaises(ValueError):
            load_config(unknown)

    def test_load_config_rejects_invalid_or_short_seed(self):
        invalid = valid_env()
        invalid["FLATKEY_QA_IDENTITY_SEED_B64"] = "not base64!!!"
        with self.assertRaises(ValueError):
            load_config(invalid)

        short = valid_env()
        short["FLATKEY_QA_IDENTITY_SEED_B64"] = base64.b64encode(b"short").decode("ascii")
        with self.assertRaises(ValueError):
            load_config(short)

    def test_load_config_rejects_non_ascii_decimal_run_id(self):
        env = valid_env()
        env["FLATKEY_QA_RUN_ID"] = "１２３"

        with self.assertRaises(ValueError):
            load_config(env)

    def test_load_config_rejects_non_exact_origins(self):
        for name in [
            "FLATKEY_QA_WEBSITE_ORIGIN",
            "FLATKEY_QA_CONSOLE_ORIGIN",
            "FLATKEY_QA_DOCS_ORIGIN",
        ]:
            env = valid_env()
            env[name] = env[name] + "/"
            with self.subTest(name=name):
                with self.assertRaises(ValueError):
                    load_config(env)

        env = valid_env()
        env["FLATKEY_QA_CONSOLE_ORIGIN"] = "https://staging-console.flatkey.ai:444"
        with self.assertRaises(ValueError):
            load_config(env)


if __name__ == "__main__":
    unittest.main()
