# Speckit Specify Workflow

## How to use specify (speckit) ?

Speckit is a workflow system for creating specifications, plans, and tasks for features in your PVMSS application.

### Using `/speckit.specify`

To create or update a feature specification:

```bash
/speckit.specify [feature description]
```

**Example:**

```bash
/speckit.specify Add user authentication with OAuth2 integration
```

#### What it does

1. **Creates a new branch**: Automatically generates a branch name like `5-user-auth`
2. **Generates specification**: Creates `specs/{number}-{short-name}/spec.md` with:
   - Feature overview
   - User scenarios
   - Functional requirements
   - Success criteria
   - Key entities
3. **Quality validation**: Creates checklist and validates spec completeness
4. **Handles clarifications**: Asks up to 3 critical questions if needed

#### Best Practices

- Be specific about WHAT users need, not HOW to implement
- Focus on user goals and business value
- Include constraints or requirements
- Avoid technical implementation details

#### Next Steps after `/speckit.specify`

- `/speckit.clarify` - If there are clarification questions
- `/speckit.plan` - Create technical implementation plan
- `/speckit.tasks` - Generate actionable development tasks

#### Available Speckit Commands

- `/speckit.specify` - Create/update feature specification
- `/speckit.clarify` - Ask targeted clarification questions
- `/speckit.plan` - Generate implementation plan
- `/speckit.tasks` - Create actionable tasks
- `/speckit.implement` - Execute implementation plan
- `/speckit.analyze` - Cross-artifact consistency analysis
- `/speckit.checklist` - Generate custom checklists
- `/speckit.constitution` - Update project constitution

#### Example Workflow

```bash
# 1. Create specification
/speckit.specify Add backup system for automatic VM snapshots

# 2. Clarify any questions (if needed)
/speckit.clarify

# 3. Create technical plan
/speckit.plan

# 4. Generate tasks
/speckit.tasks

# 5. Implement
/speckit.implement
```

The specification files are stored in `specs/` directory with numbered folders for each feature.
