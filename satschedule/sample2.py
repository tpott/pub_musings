# written with claude 3.7 sonnet web UI with the following prompt:
# Give an example input for the SMT solver Z3 that highlights how it can be used
# for re-scheduling a calendar to make room for a meeting? Like, if someone asks
# to meet with you and provides three possibilities that otherwise show conflicts
# with your calendar, try removing one of those conflicting meetings, and see if
# you can find a different time (during regular working hours for requester and
# requestee) for that meeting.

import sys

from z3 import *

# Create a Z3 solver
solver = Solver()

# Define time slots (hours in a work day: 9am-5pm)
TIME_SLOTS = 8
DAYS = 5  # Monday to Friday
WORKING_HOURS_START = 9  # 9am

# Creating boolean variables for each time slot in the week
# your_calendar[day][hour] = True means you have a meeting at that time
your_calendar = [
    [Bool(f"your_meeting_day{d}_hour{h}") for h in range(TIME_SLOTS)]
    for d in range(DAYS)
]

# The requester's calendar (True means they're busy)
requester_calendar = [
    [Bool(f"requester_busy_day{d}_hour{h}") for h in range(TIME_SLOTS)]
    for d in range(DAYS)
]

# Define your current meetings (True = busy)
# Monday meetings
solver.add(your_calendar[0][0] == True)  # Monday 9am
solver.add(your_calendar[0][1] == True)  # Monday 10am
solver.add(your_calendar[0][2] == False)  # Monday 11am
solver.add(your_calendar[0][3] == True)  # Monday 12pm
solver.add(your_calendar[0][4] == False)  # Monday 1pm
solver.add(your_calendar[0][5] == False)  # Monday 2pm
solver.add(your_calendar[0][6] == True)  # Monday 3pm
solver.add(your_calendar[0][7] == True)  # Monday 4pm

# Tuesday meetings
solver.add(your_calendar[1][0] == False)  # Tuesday 9am
solver.add(your_calendar[1][1] == True)  # Tuesday 10am
solver.add(your_calendar[1][2] == True)  # Tuesday 11am - THIS IS MEETING A
solver.add(your_calendar[1][3] == False)  # Tuesday 12pm
solver.add(your_calendar[1][4] == False)  # Tuesday 1pm
solver.add(your_calendar[1][5] == True)  # Tuesday 2pm
solver.add(your_calendar[1][6] == True)  # Tuesday 3pm
solver.add(your_calendar[1][7] == False)  # Tuesday 4pm

# Wednesday meetings
solver.add(your_calendar[2][0] == True)  # Wednesday 9am
solver.add(your_calendar[2][1] == True)  # Wednesday 10am - THIS IS MEETING B
solver.add(your_calendar[2][2] == False)  # Wednesday 11am
solver.add(your_calendar[2][3] == False)  # Wednesday 12pm
solver.add(your_calendar[2][4] == True)  # Wednesday 1pm
solver.add(your_calendar[2][5] == True)  # Wednesday 2pm
solver.add(your_calendar[2][6] == False)  # Wednesday 3pm
solver.add(your_calendar[2][7] == False)  # Wednesday 4pm

# Thursday meetings
solver.add(your_calendar[3][0] == False)  # Thursday 9am
solver.add(your_calendar[3][1] == False)  # Thursday 10am
solver.add(your_calendar[3][2] == True)  # Thursday 11am
solver.add(your_calendar[3][3] == True)  # Thursday 12pm
solver.add(your_calendar[3][4] == True)  # Thursday 1pm - THIS IS MEETING C
solver.add(your_calendar[3][5] == False)  # Thursday 2pm
solver.add(your_calendar[3][6] == False)  # Thursday 3pm
solver.add(your_calendar[3][7] == True)  # Thursday 4pm

# Friday meetings
solver.add(your_calendar[4][0] == True)  # Friday 9am
solver.add(your_calendar[4][1] == False)  # Friday 10am
solver.add(your_calendar[4][2] == False)  # Friday 11am
solver.add(your_calendar[4][3] == True)  # Friday 12pm
solver.add(your_calendar[4][4] == True)  # Friday 1pm
solver.add(your_calendar[4][5] == True)  # Friday 2pm
solver.add(your_calendar[4][6] == True)  # Friday 3pm
solver.add(your_calendar[4][7] == False)  # Friday 4pm

# Define the requester's availability (True = busy)
# Assume they're busy in these slots
for d in range(DAYS):
    for h in range(TIME_SLOTS):
        # Default is available, we'll specify busy times
        solver.add(requester_calendar[d][h] == False)

# Requester busy times
solver.add(requester_calendar[0][6] == True)  # Monday 3pm
solver.add(requester_calendar[0][7] == True)  # Monday 4pm
solver.add(requester_calendar[1][0] == True)  # Tuesday 9am
solver.add(requester_calendar[1][1] == True)  # Tuesday 10am
solver.add(requester_calendar[2][4] == True)  # Wednesday 1pm
solver.add(requester_calendar[2][5] == True)  # Wednesday 2pm
solver.add(requester_calendar[2][6] == True)  # Wednesday 3pm
solver.add(requester_calendar[3][0] == True)  # Thursday 9am
solver.add(requester_calendar[4][4] == True)  # Friday 1pm
solver.add(requester_calendar[4][5] == True)  # Friday 2pm
solver.add(requester_calendar[4][6] == True)  # Friday 3pm
solver.add(requester_calendar[4][7] == True)  # Friday 4pm

# The three possible meeting times proposed by requester (all currently have conflicts)
proposed_slots = [
    (1, 2),  # Tuesday 11am (conflicts with MEETING A)
    (2, 1),  # Wednesday 10am (conflicts with MEETING B)
    (3, 4),  # Thursday 1pm (conflicts with MEETING C)
]

# Create variables to track which meeting could be rescheduled
meeting_a_moved = Bool("meeting_a_moved")
meeting_b_moved = Bool("meeting_b_moved")
meeting_c_moved = Bool("meeting_c_moved")

# Create variables to track where each meeting might be rescheduled
# Day and hour for each rescheduled meeting
meeting_a_new_day = Int("meeting_a_new_day")
meeting_a_new_hour = Int("meeting_a_new_hour")
meeting_b_new_day = Int("meeting_b_new_day")
meeting_b_new_hour = Int("meeting_b_new_hour")
meeting_c_new_day = Int("meeting_c_new_day")
meeting_c_new_hour = Int("meeting_c_new_hour")

# New variable to determine which proposed slot to accept
use_proposal_1 = Bool("use_proposal_1")
use_proposal_2 = Bool("use_proposal_2")
use_proposal_3 = Bool("use_proposal_3")

# Constraint: Accept exactly one proposal
solver.add(PbEq([(use_proposal_1, 1), (use_proposal_2, 1), (use_proposal_3, 1)], 1))

# Constraint: Move at most one meeting
solver.add(PbLe([(meeting_a_moved, 1), (meeting_b_moved, 1), (meeting_c_moved, 1)], 1))

# If we accept proposal 1, we need to move Meeting A
solver.add(Implies(use_proposal_1, meeting_a_moved))

# If we accept proposal 2, we need to move Meeting B
solver.add(Implies(use_proposal_2, meeting_b_moved))

# If we accept proposal 3, we need to move Meeting C
solver.add(Implies(use_proposal_3, meeting_c_moved))

# Constrain new meeting times to valid values
for meeting_day, meeting_hour in [
    (meeting_a_new_day, meeting_a_new_hour),
    (meeting_b_new_day, meeting_b_new_hour),
    (meeting_c_new_day, meeting_c_new_hour),
]:
    solver.add(And(0 <= meeting_day, meeting_day < DAYS))
    solver.add(And(0 <= meeting_hour, meeting_hour < TIME_SLOTS))


# Ensure moved meetings don't conflict with existing meetings or requester's schedule
# Function to check if a time slot is free for both parties
def no_conflict_at(day_val, hour_val):
    # We need concrete integer values for indexing
    day_idx = day_val.as_long() if hasattr(day_val, "as_long") else day_val
    hour_idx = hour_val.as_long() if hasattr(hour_val, "as_long") else hour_val

    # Create a constraint that this time is available
    # First, check bounds
    if not (0 <= day_idx < DAYS and 0 <= hour_idx < TIME_SLOTS):
        return False

    # Check if this slot is free in both calendars
    if day_idx == 1 and hour_idx == 1:  # Meeting A slot
        return And(
            Or(Not(your_calendar[day_idx][hour_idx]), meeting_a_moved),
            Not(requester_calendar[day_idx][hour_idx]),
        )
    elif day_idx == 2 and hour_idx == 1:  # Meeting B slot
        return And(
            Or(Not(your_calendar[day_idx][hour_idx]), meeting_b_moved),
            Not(requester_calendar[day_idx][hour_idx]),
        )
    elif day_idx == 3 and hour_idx == 4:  # Meeting C slot
        return And(
            Or(Not(your_calendar[day_idx][hour_idx]), meeting_c_moved),
            Not(requester_calendar[day_idx][hour_idx]),
        )
    else:
        return And(
            Not(your_calendar[day_idx][hour_idx]),
            Not(requester_calendar[day_idx][hour_idx]),
        )


# For the new meeting times, we need a different approach
# Z3 allows us to use If and functions to handle this


# Create a function to check if a slot is free in your calendar
def is_free_in_your_calendar(day_idx, hour_idx):
    result = True
    # Check all meeting slots
    for d in range(DAYS):
        for h in range(TIME_SLOTS):
            # For the meetings that might be moved, use Z3's If expression
            meeting_check = True
            if d == 1 and h == 1:  # Meeting A
                meeting_check = Implies(meeting_a_moved, True)
            elif d == 2 and h == 1:  # Meeting B
                meeting_check = Implies(meeting_b_moved, True)
            elif d == 3 and h == 4:  # Meeting C
                meeting_check = Implies(meeting_c_moved, True)

            # Only constrain this slot if it's not a moved meeting
            result = And(
                result,
                Implies(
                    And(day_idx == d, hour_idx == h),
                    Or(meeting_check, Not(your_calendar[d][h])),
                ),
            )
    return result


# Create a function to check if a slot is free in requester's calendar
def is_free_in_requester_calendar(day_idx, hour_idx):
    result = True
    for d in range(DAYS):
        for h in range(TIME_SLOTS):
            result = And(
                result,
                Implies(
                    And(day_idx == d, hour_idx == h), Not(requester_calendar[d][h])
                ),
            )
    return result


# Add constraint for Meeting A's new time
solver.add(
    Implies(
        meeting_a_moved,
        And(
            is_free_in_your_calendar(meeting_a_new_day, meeting_a_new_hour),
            is_free_in_requester_calendar(meeting_a_new_day, meeting_a_new_hour),
        ),
    )
)

# Add constraint for Meeting B's new time
solver.add(
    Implies(
        meeting_b_moved,
        And(
            is_free_in_your_calendar(meeting_b_new_day, meeting_b_new_hour),
            is_free_in_requester_calendar(meeting_b_new_day, meeting_b_new_hour),
        ),
    )
)

# Add constraint for Meeting C's new time
solver.add(
    Implies(
        meeting_c_moved,
        And(
            is_free_in_your_calendar(meeting_c_new_day, meeting_c_new_hour),
            is_free_in_requester_calendar(meeting_c_new_day, meeting_c_new_hour),
        ),
    )
)

# Make sure we don't double-book the moved meetings
# This part doesn't need to be changed since we're using Z3's equality operators
# which work fine with symbolic variables
solver.add(
    Implies(
        And(meeting_a_moved, meeting_b_moved),
        Not(
            And(
                meeting_a_new_day == meeting_b_new_day,
                meeting_a_new_hour == meeting_b_new_hour,
            )
        ),
    )
)
solver.add(
    Implies(
        And(meeting_a_moved, meeting_c_moved),
        Not(
            And(
                meeting_a_new_day == meeting_c_new_day,
                meeting_a_new_hour == meeting_c_new_hour,
            )
        ),
    )
)
solver.add(
    Implies(
        And(meeting_b_moved, meeting_c_moved),
        Not(
            And(
                meeting_b_new_day == meeting_c_new_day,
                meeting_b_new_hour == meeting_c_new_hour,
            )
        ),
    )
)

# Ensure moved meetings aren't placed in the slots that we're trying to free up
# Don't move Meeting A to a slot that has one of the other proposed meetings
solver.add(
    Implies(meeting_a_moved, Not(And(meeting_a_new_day == 2, meeting_a_new_hour == 1)))
)  # Not Meeting B's slot
solver.add(
    Implies(meeting_a_moved, Not(And(meeting_a_new_day == 3, meeting_a_new_hour == 4)))
)  # Not Meeting C's slot

# Don't move Meeting B to other proposed meeting slots
solver.add(
    Implies(meeting_b_moved, Not(And(meeting_b_new_day == 1, meeting_b_new_hour == 1)))
)  # Not Meeting A's slot
solver.add(
    Implies(meeting_b_moved, Not(And(meeting_b_new_day == 3, meeting_b_new_hour == 4)))
)  # Not Meeting C's slot

# Don't move Meeting C to other proposed meeting slots
solver.add(
    Implies(meeting_c_moved, Not(And(meeting_c_new_day == 1, meeting_c_new_hour == 1)))
)  # Not Meeting A's slot
solver.add(
    Implies(meeting_c_moved, Not(And(meeting_c_new_day == 2, meeting_c_new_hour == 1)))
)  # Not Meeting B's slot

# Check if there is a solution
if solver.check() != sat:
    print(
        "No solution found. Cannot reschedule meetings to accommodate any of the proposals."
    )
    sys.exit(1)

model = solver.model()


# Helper function to safely evaluate boolean expressions
def is_true(expr):
    result = model.evaluate(expr)
    return result == True


# See which proposal was accepted
proposal_accepted = None
if is_true(use_proposal_1):
    proposal_accepted = "Proposal 1 (Tuesday 10am)"
elif is_true(use_proposal_2):
    proposal_accepted = "Proposal 2 (Wednesday 10am)"
elif is_true(use_proposal_3):
    proposal_accepted = "Proposal 3 (Thursday 1pm)"

print(f"Solution found! Accepting: {proposal_accepted}")


# Helper function to convert Z3 values to Python integers
def get_int_value(expr):
    val = model.evaluate(expr)
    if hasattr(val, "as_long"):
        return val.as_long()
    return val


day_names = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]

# See which meeting was moved
if is_true(meeting_a_moved):
    new_day = get_int_value(meeting_a_new_day)
    new_hour = get_int_value(meeting_a_new_hour)
    print(
        f"Meeting A moved from Tuesday 10am to {day_names[new_day]} {new_hour+WORKING_HOURS_START}:00"
    )
elif is_true(meeting_b_moved):
    new_day = get_int_value(meeting_b_new_day)
    new_hour = get_int_value(meeting_b_new_hour)
    print(
        f"Meeting B moved from Wednesday 10am to {day_names[new_day]} {new_hour+WORKING_HOURS_START}:00"
    )
elif is_true(meeting_c_moved):
    new_day = get_int_value(meeting_c_new_day)
    new_hour = get_int_value(meeting_c_new_hour)
    print(
        f"Meeting C moved from Thursday 1pm to {day_names[new_day]} {new_hour+WORKING_HOURS_START}:00"
    )

# Print a visual representation of the new schedule
print("\nSchedule after rescheduling:")
print("---------------------------")
for d in range(DAYS):
    print(f"{day_names[d]}:")
    for h in range(TIME_SLOTS):
        hour_str = f"{h+WORKING_HOURS_START}:00"
        status = "FREE"

        # Check your original meetings
        if model.evaluate(your_calendar[d][h]) == True:
            # Skip the meeting that was moved
            if (
                d == 1 and h == 1 and model.evaluate(meeting_a_moved) == True
            ):  # Meeting A
                pass
            elif (
                d == 2 and h == 1 and model.evaluate(meeting_b_moved) == True
            ):  # Meeting B
                pass
            elif (
                d == 3 and h == 4 and model.evaluate(meeting_c_moved) == True
            ):  # Meeting C
                pass
            else:
                status = "BUSY"

        # Check if this is the new meeting with the requester
        if (
            (d == 1 and h == 1 and model.evaluate(use_proposal_1) == True)
            or (d == 2 and h == 1 and model.evaluate(use_proposal_2) == True)
            or (d == 3 and h == 4 and model.evaluate(use_proposal_3) == True)
        ):
            status = "NEW MEETING"

        # Check if this is where one of the moved meetings was rescheduled
        moved_a = model.evaluate(meeting_a_moved) == True
        moved_b = model.evaluate(meeting_b_moved) == True
        moved_c = model.evaluate(meeting_c_moved) == True

        if (
            moved_a
            and get_int_value(meeting_a_new_day) == d
            and get_int_value(meeting_a_new_hour) == h
        ):
            status = "MOVED A"
        elif (
            moved_b
            and get_int_value(meeting_b_new_day) == d
            and get_int_value(meeting_b_new_hour) == h
        ):
            status = "MOVED B"
        elif (
            moved_c
            and get_int_value(meeting_c_new_day) == d
            and get_int_value(meeting_c_new_hour) == h
        ):
            status = "MOVED C"

        print(f"  {hour_str}: {status}")
