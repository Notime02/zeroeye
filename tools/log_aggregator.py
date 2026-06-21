import json
import unittest

class JSONLogParser:
    # Implementation of JSON log parser
    pass

class TextLogParser:
    # Implementation of text log parser
    pass

class NginxLogParser:
    # Implementation of Nginx log parser
    pass

class TestLogParsers(unittest.TestCase):

    def test_valid_json_log(self):
        log_line = '{"timestamp": "2023-10-01T12:00:00Z", "level": "INFO", "service": "my_service", "message": "This is a log message."}'
        parser = JSONLogParser()
        result = parser.parse(log_line)
        self.assertEqual(result['timestamp'], '2023-10-01T12:00:00Z')
        self.assertEqual(result['level'], 'INFO')
        self.assertEqual(result['service'], 'my_service')
        self.assertEqual(result['message'], 'This is a log message.')

    def test_malformed_json_log(self):
        log_line = '{"timestamp": "2023-10-01T12:00:00Z", "level": "INFO", "service": "my_service", "message": "This is a log message."
        parser = JSONLogParser()
        with self.assertRaises(ValueError):
            parser.parse(log_line)

    def test_empty_line(self):
        log_line = ''
        parser = TextLogParser()
        result = parser.parse(log_line)
        self.assertIsNone(result)

    def test_nginx_access_log(self):
        log_line = '127.0.0.1 - - [01/Oct/2023:12:00:00 +0000] "GET / HTTP/1.1" 200 2326'
        parser = NginxLogParser()
        result = parser.parse(log_line)
        self.assertEqual(result['status'], 200)
        self.assertEqual(result['method'], 'GET')

    def test_unknown_severity(self):
        log_line = '{"timestamp": "2023-10-01T12:00:00Z", "level": "UNKNOWN", "service": "my_service", "message": "This is a log message."}'
        parser = JSONLogParser()
        result = parser.parse(log_line)
        self.assertEqual(result['level'], 'UNKNOWN')

    def test_http_error_codes(self):
        log_line = '{"timestamp": "2023-10-01T12:00:00Z", "level": "ERROR", "service": "my_service", "message": "HTTP 500 error occurred."}'
        parser = JSONLogParser()
        result = parser.parse(log_line)
        self.assertEqual(result['level'], 'ERROR')

if __name__ == '__main__':
    unittest.main()