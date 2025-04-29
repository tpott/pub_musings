# written with claude 3.7 sonnet web UI with the following prompt:
# Give an example input for the SMT solver Z3 that highlights how it can be used
# for scheduling/planning a large software and hardware project?

from z3 import *

# Create a solver instance
solver = Solver()

# Define time range for the project (days)
max_days = 120

# Define tasks with estimated durations (in days)
tasks = {
    "requirements_analysis": 10,
    "hardware_design": 20,
    "hardware_prototyping": 15,
    "hardware_testing": 10,
    "firmware_development": 25,
    "software_architecture": 12,
    "frontend_development": 30,
    "backend_development": 35,
    "integration_testing": 15,
    "system_testing": 20,
    "documentation": 15,
    "user_acceptance_testing": 10,
}

# Create variables for start times of each task
start_times = {task: Int(f"start_{task}") for task in tasks}

# Add constraints that all tasks must start within project timeframe
for task in tasks:
    solver.add(start_times[task] >= 0)
    solver.add(start_times[task] + tasks[task] <= max_days)

# Define dependencies between tasks
# Format: task_name: [list of prerequisite tasks]
dependencies = {
    "hardware_design": ["requirements_analysis"],
    "hardware_prototyping": ["hardware_design"],
    "hardware_testing": ["hardware_prototyping"],
    "firmware_development": ["hardware_design"],
    "software_architecture": ["requirements_analysis"],
    "frontend_development": ["software_architecture"],
    "backend_development": ["software_architecture"],
    "integration_testing": [
        "frontend_development",
        "backend_development",
        "firmware_development",
    ],
    "system_testing": ["integration_testing", "hardware_testing"],
    "documentation": ["system_testing"],
    "user_acceptance_testing": ["system_testing", "documentation"],
}

# Add dependency constraints
for task, prereqs in dependencies.items():
    for prereq in prereqs:
        solver.add(start_times[task] >= start_times[prereq] + tasks[prereq])

# Define available resources (engineers)
resources = {
    "hardware_engineers": 3,
    "firmware_engineers": 2,
    "backend_developers": 4,
    "frontend_developers": 3,
    "qa_engineers": 2,
}

# Define which resources are needed for which tasks
task_resources = {
    "requirements_analysis": {"hardware_engineers": 1, "backend_developers": 1},
    "hardware_design": {"hardware_engineers": 3},
    "hardware_prototyping": {"hardware_engineers": 2},
    "hardware_testing": {"hardware_engineers": 1, "qa_engineers": 1},
    "firmware_development": {"firmware_engineers": 2},
    "software_architecture": {"backend_developers": 2},
    "frontend_development": {"frontend_developers": 3},
    "backend_development": {"backend_developers": 4},
    "integration_testing": {
        "firmware_engineers": 1,
        "frontend_developers": 1,
        "backend_developers": 1,
    },
    "system_testing": {"qa_engineers": 2, "hardware_engineers": 1},
    "documentation": {
        "backend_developers": 1,
        "frontend_developers": 1,
        "hardware_engineers": 1,
    },
    "user_acceptance_testing": {"qa_engineers": 2},
}

# Create Boolean variables for each day and task to track when tasks are active
task_active = {}
for task in tasks:
    task_active[task] = [Bool(f"{task}_active_day_{day}") for day in range(max_days)]

    # Set when the task is active
    for day in range(max_days):
        solver.add(
            task_active[task][day]
            == And(day >= start_times[task], day < start_times[task] + tasks[task])
        )

# Ensure resource constraints are not violated for each day
for day in range(max_days):
    for resource in resources:
        # Calculate total usage of this resource on this day
        usage_expr = Sum(
            [
                If(task_active[task][day], task_resources[task].get(resource, 0), 0)
                for task in tasks
                if resource in task_resources.get(task, {})
            ]
        )

        # Ensure we don't exceed available resources
        solver.add(usage_expr <= resources[resource])

# Optimization goal: minimize project duration
project_end = Int("project_end")
for task in tasks:
    solver.add(project_end >= start_times[task] + tasks[task])

# Set the objective to minimize project end time
optimize = Optimize()
optimize.add(solver.assertions())
optimize.minimize(project_end)

# Check if the constraints can be satisfied
if optimize.check() == sat:
    model = optimize.model()

    print(f"Project can be completed in {model.evaluate(project_end)} days")
    print("\nTask Schedule:")
    for task in sorted(tasks, key=lambda t: model.evaluate(start_times[t]).as_long()):
        start = model.evaluate(start_times[task]).as_long()
        duration = tasks[task]
        end = start + duration
        print(
            f"{task}: Start on day {start}, End on day {end} (Duration: {duration} days)"
        )
else:
    print("No feasible schedule found!")
