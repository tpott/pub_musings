import ast
import textwrap
import sys


# Visitor to track nesting depths
class NestingVisitor(ast.NodeVisitor):
    def __init__(self):
        self.current_path = []
        self.max_depth_seen = 0
        self.current_function = None
        self.problematic_nodes = []

    def visit_FunctionDef(self, node):
        old_function = self.current_function
        self.current_function = node.name
        self.generic_visit(node)
        self.current_function = old_function

    def visit_If(self, node):
        self.current_path.append(node)
        nesting_depth = self._count_nested_ifs()

        if nesting_depth > self.max_depth_seen:
            self.max_depth_seen = nesting_depth

        if nesting_depth > max_depth and self.current_function:
            if self.current_function not in [p[0] for p in self.problematic_nodes]:
                self.problematic_nodes.append(
                    (self.current_function, nesting_depth, node.lineno)
                )

        # Visit children
        for child in ast.iter_child_nodes(node):
            self.visit(child)

        self.current_path.pop()

    # Also track For, While and With statements as they contribute to nesting
    visit_For = visit_If
    visit_While = visit_If
    visit_With = visit_If

    def _count_nested_ifs(self):
        """Count how many nested control flow statements we're in"""
        return len(self.current_path)


def check_nesting_depth(code, max_depth=3):
    """
    Analyze Python code for excessive nesting.

    Args:
        code: String containing Python code
        max_depth: Maximum acceptable nesting depth

    Returns:
        Dictionary with nesting info and problematic functions
    """
    parsed = ast.parse(code)
    result = {"excessive_nesting": False, "problematic_functions": []}

    visitor = NestingVisitor()
    visitor.visit(parsed)

    # Format results
    if visitor.problematic_nodes:
        result["excessive_nesting"] = True
        for func_name, depth, line_no in visitor.problematic_nodes:
            result["problematic_functions"].append(
                {"function": func_name, "max_depth": depth, "line_number": line_no}
            )

    return result


# Example usage
if __name__ == "__main__":
    test_code = """
def test_function():
    for i in range(10):
        if i > 5:
            if i % 2 == 0:
                if i % 3 == 0:
                    print("Nested too deep!")

def good_function():
    for i in range(10):
        if i > 5:
            continue
        if i % 2 == 0:
            print("Even number")
        elif i % 3 == 0:
            print("Divisible by 3")
    """
    if len(sys.argv) <= 1:
        result = check_nesting_depth(test_code)
        print(result)
        sys.exit(0)
    result = check_nesting_depth(open(sys.argv[1]).read())
    print(result)
