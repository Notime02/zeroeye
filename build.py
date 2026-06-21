import subprocess
from typing import Optional

# ... (truncated) ...

def build():
    # ... (truncated) ...
    add = subprocess.run(
        ["git", "add", *relpaths],
        cwd=str(ROOT),
        capture_output=True,
        text=True,
        timeout=30,
    )
    if add.returncode != 0:
        print(f"    {color('✗', Colors.RED)} Could not stage diagnostic artifacts: {add.stderr.strip()}")
        return False

    commit = subprocess.run(
        ["git", "commit", "-m", f"Add build diagnostics for {commit_id}", "--", *relpaths],
        cwd=str(ROOT),
        capture_output=True,
        text=True,
        timeout=600,
    )
    if commit.returncode != 0:
        output = commit.stderr.strip() or commit.stdout.strip()
        print(f"    {color('✗', Colors.RED)} Could not commit diagnostic artifacts: {output}")
        return False

    print(f"    {color('✓', Colors.GREEN)} Diagnostic artifacts committed")
    return True


def generate_logd(
    results: list[tuple[str, bool, float, str, Optional[str]]],
    verbose: bool = False,
) -> bool:
    logd_path, metadata_path, commit_id = diagnostic_paths_for_commit()
    display_logd = logd_path.relative_to(ROOT)
    print(f"\n  {color('▸', Colors.CYAN)} Finalizing diagnostics for {color(str(display_logd), Colors.BOLD)}...")

    # Always write the JSON report first. The encrypted .logd is useful, but the
    # report is required even when the build failed before compilation started or
    # when encryptly itself is unavailable.
    write_diagnostic_report(metadata_path, build_diagnostic_report(results, commit_id))

    encryptly_bin = get_encryptly_bin()
    if encryptly_bin is None:
        print(f"    {color('✗', Colors.RED)} Encryptly binary not found. Skipping .logd generation.")
        return False

    # ... (truncated) ...
