import argparse
import json

# Define the command-line arguments
parser = argparse.ArgumentParser(description="Extract unique 'CUDA' values from JSON files")
parser.add_argument("file1", help="Path to the first JSON file")
parser.add_argument("file2", help="Path to the second JSON file")
args = parser.parse_args()

# Read JSON data from the specified files
with open(args.file1, 'r') as file1:
    json_data1 = json.load(file1)

with open(args.file2, 'r') as file2:
    json_data2 = json.load(file2)

# Combine both JSON data
combined_data = json_data1 + json_data2

# Extract unique "CUDA" values
unique_cuda_values = set(item.get("CUDA") for item in combined_data if item.get("CUDA") is not None)

# Print the unique CUDA values
for cuda_value in unique_cuda_values:
    print(cuda_value)

