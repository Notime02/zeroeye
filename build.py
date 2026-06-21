import subprocess
import shutil
from typing import Optional

# ... (other imports and code) ...

def clean_module(module_name: str, clean_command: str, verbose: bool) -> bool:
    try:
        print(f'Cleaning module: {module_name}')
        result = subprocess.run(clean_command, shell=True, capture_output=True, text=True)
        if result.returncode != 0:
            print(f'Error cleaning module {module_name}:')
            print(f'Stdout: {result.stdout}')
            print(f'Stderr: {result.stderr}')
            return False
        if verbose:
            print(f'Successfully cleaned module: {module_name}')
        return True
    except Exception as e:
        print(f'Exception occurred while cleaning module {module_name}: {e}')
        return False
    finally:
        shutil.rmtree(workspace, ignore_errors=True)

# ... (rest of the code) ...

def print_summary(results: list[tuple[str, bool, float, str, Optional[str]]]):
    print(f'  {color('Build Summary', Colors.BOLD)}')

    total = len(results)
    passed = sum(1 for _, s, _, _, _ in results if s)
    failed = total - passed
    total_time = sum(t for _, _, t, _, _ in results)

    for name, success, elapsed, output, binary in results:
        status_icon = color('✓', Colors.GREEN) if success else color('✗', Colors.RED)
        status_text = color('PASS', Colors.GREEN) if success else color('FAIL', Colors.RED)
        time_str = f'{elapsed:.1f}s' if elapsed < 60 else f'{elapsed / 60:.1f}m'

        print(f'\n  {status_icon}  {color(name + ':', Colors.BOLD)} {status_text}  ({time_str})')
        if binary:
            print(f'       artifact: {color(binary, Colors.GRAY)}')
# ... (truncated) ...