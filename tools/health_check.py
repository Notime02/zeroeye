import time
import json
from typing import Dict, Any

class HealthCheck:
    def __init__(self):
        self.metrics = {}

    def add_metric(self, name: str, timestamp: float, value: Any):
        self.metrics[name] = {'timestamp': timestamp, 'value': value}

    def check_stale_metrics(self, threshold: float) -> Dict[str, Any]:
        stale_metrics = []
        current_time = time.time()
        for name, metric in self.metrics.items():
            age = current_time - metric['timestamp']
            if age > threshold:
                stale_metrics.append({
                    'service': 'my_service',
                    'environment': 'production',
                    'metric_name': name,
                    'timestamp': metric['timestamp'],
                    'stale_status': 'stale',
                    'age': age
                })
        return stale_metrics

    def generate_health_output(self) -> str:
        stale_metrics = self.check_stale_metrics(threshold=300)  # 5 minutes threshold
        output = {
            'current_metrics': self.metrics,
            'stale_metrics': stale_metrics
        }
        return json.dumps(output, indent=4)

# Example usage
health_check = HealthCheck()
health_check.add_metric('cpu_usage', time.time(), 75)
health_check.add_metric('memory_usage', time.time() - 400, 60)  # Stale metric
print(health_check.generate_health_output())