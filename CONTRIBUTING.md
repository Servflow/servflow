# Contributing to Servflow

Thank you for your interest in contributing to Servflow! This document provides guidelines for contributing to this repository, which serves as the public distribution point and issue tracker for Servflow.

## 📋 How to Contribute

### Reporting Issues

This repository is primarily for:

- 🐞 **Bug Reports**: Issues with Servflow functionality
- 💡 **Feature Requests**: Suggestions for new features
- 🔄 **Action Requests**: Requests for new workflow actions
- 💬 **General Feedback**: Questions, suggestions, and user experience feedback

### Before Creating an Issue

1. **Search Existing Issues**: Check if your issue has already been reported
2. **Check Documentation**: Review the [documentation](https://docs.servflow.io) first
3. **Use the Right Template**: Choose the appropriate issue template for your report

### Issue Guidelines

#### Bug Reports
- Use a clear, descriptive title
- Include Servflow version and installation method
- Provide detailed steps to reproduce
- Include relevant logs, screenshots, or trace IDs
- Specify your environment (OS, browser, etc.)

#### Feature Requests
- Clearly describe the proposed feature
- Explain the problem it would solve
- Provide concrete use cases
- Indicate priority level

#### Action Requests
- Suggest a clear name for the action
- Define required input fields and output format
- Explain when and how you'd use it
- Provide real-world examples

### Issue Labels

We use these labels to organize issues:

**Type:**
- `bug` - Something isn't working
- `enhancement` - New feature request
- `action-request` - New workflow action request
- `feedback` - General feedback or questions
- `documentation` - Documentation improvements

**Priority:**
- `high-priority` - Critical issues requiring immediate attention
- `low-priority` - Nice-to-have improvements

**Status:**
- `investigating` - Being looked into by the team
- `planned` - Accepted and planned for implementation
- `help-wanted` - Community contributions welcome
- `duplicate` - Already reported elsewhere
- `wontfix` - Not planned for implementation

## 🚫 What This Repository Is NOT For

- **Source Code**: This is a binary distribution repo, not the source code
- **Private Support**: Use [support@servflow.com](mailto:support@servflow.com) for private issues
- **Security Vulnerabilities**: Report security issues privately to [security@servflow.com](mailto:security@servflow.com)
- **General Questions**: Use [GitHub Discussions](https://github.com/servflow/servflow/discussions) or [documentation](https://docs.servflow.io)

## 🏷️ Issue Triage Process

The Servflow team follows this triage process:

1. **Initial Review** (within 48 hours)
   - Verify issue template completion
   - Add appropriate labels
   - Ask for clarification if needed

2. **Investigation** (within 1 week)
   - Attempt to reproduce bugs
   - Assess feature request feasibility
   - Prioritize based on impact and effort

3. **Resolution Planning**
   - Schedule for upcoming releases
   - Mark as `help-wanted` if suitable for community contributions
   - Close if duplicate or not actionable

## 📝 Writing Good Issues

### Good Bug Report Example
```
Title: [BUG] Workflow fails to save when using MySQL action with special characters

Description:
When creating a workflow that includes a MySQL action with SQL queries containing special characters (like apostrophes), the workflow fails to save and shows a validation error.

Steps to Reproduce:
1. Create new workflow
2. Add MySQL action
3. Set query to: SELECT * FROM users WHERE name = 'O'Brien'
4. Try to save workflow

Expected: Workflow saves successfully
Actual: Shows "Invalid query format" error

Environment:
- Servflow Version: v1.2.3
- Installation: Docker
- OS: Ubuntu 22.04
- Browser: Chrome 120.0
```

### Good Feature Request Example
```
Title: [FEATURE] Add batch processing support for large datasets

Description:
Add ability to process large datasets in configurable batches to prevent memory issues and improve performance.

Problem It Solves:
Currently, processing datasets with >10k records causes memory issues and timeouts.

Use Case:
I need to process CSV files with 100k+ customer records through multiple workflow steps.

Priority: [x] Would significantly improve my workflow
```

## 🤝 Community Guidelines

- **Be Respectful**: Treat everyone with respect and kindness
- **Be Constructive**: Provide helpful, actionable feedback
- **Be Patient**: The team reviews issues regularly but may need time to respond
- **Stay On Topic**: Keep discussions focused on the specific issue
- **Follow Templates**: Use the provided issue templates completely

## 📞 Getting Help

- **Documentation**: [docs.servflow.io](https://docs.servflow.io)
- **Community Discussions**: [GitHub Discussions](https://github.com/servflow/servflow/discussions)
- **Support**: [support@servflow.com](mailto:support@servflow.com)
- **Security Issues**: [security@servflow.com](mailto:security@servflow.com)

## 🙏 Recognition

We appreciate all contributions to making Servflow better! Contributors who provide valuable feedback, detailed bug reports, or great feature suggestions may be recognized in our release notes.

---

Thank you for helping us improve Servflow! 🚀