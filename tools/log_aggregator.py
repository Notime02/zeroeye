import re
import unittest
from pathlib import Path

# Load actual production logs
log_path = Path('path/to/production/logs.log')
logs = log_path.read_text().splitlines()

# Define the regex pattern for the log parser
pattern = re.compile(r'^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\] (\w+) (\w+): (.*)$')

# Define the test suite
class TestLogParser(unittest.TestCase):
    def test_log_parser(self):
        for log in logs:
            match = pattern.match(log)
            if match:
                self.assertIsNotNone(match.group(1))  # timestamp
                self.assertIsNotNone(match.group(2))  # log level
                self.assertIsNotNone(match.group(3))  # module
                self.assertIsNotNone(match.group(4))  # message
            else:
                self.fail(f'Failed to parse log: {log}')

    def test_edge_cases(self):
        # Test malformed lines
        malformed_logs = [
            'Invalid log format',
            '[2022-01-01 12:00:00] INFO',
            '[2022-01-01 12:00:00] INFO module',
        ]
        for log in malformed_logs:
            match = pattern.match(log)
            self.assertIsNone(match)

if __name__ == '__main__':
    unittest.main()