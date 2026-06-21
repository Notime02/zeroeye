import re

# ... (other imports) ...

# Function to validate Terraform resource names

def validate_resource_name(resource_name):
    # Terraform resource names must match the regex pattern
    pattern = r'^[a-zA-Z0-9_]+$'
    if not re.match(pattern, resource_name):
        return False
    return True

# ... (other code) ...

# In the function where resources are processed for import
for resource in resources_to_import:
    resource_name = resource.resource_name
    if not validate_resource_name(resource_name):
        print(f"Error: Invalid resource name '{resource_name}' for resource type '{resource.resource_type}'. Resource names must consist of alphanumeric characters and underscores only.")
        continue  # Skip this resource and move to the next

    # Proceed with the import if the name is valid
    # ... (existing import logic) ...

# ... (rest of the file) ...