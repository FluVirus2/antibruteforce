# Anti-Bruteforce Service Documentation

## Quick Start
- **[Subnet Quick Start Guide](SUBNET_QUICKSTART.md)** - Get started with IP subnet whitelist/blacklist system

## Architecture
- **[Architecture Overview](ARCHITECTURE.md)** - Layered architecture and design patterns
- **[Clean Architecture](CLEAN_ARCHITECTURE.md)** - Single Responsibility Principle explained
- **[Separation of Concerns](SEPARATION_OF_CONCERNS.md)** - How we properly separated layers
- **[Subnet System Design](SUBNET_SYSTEM_DESIGN.md)** - IP subnet checking system design details
- **[Refactoring Summary](REFACTORING_SUMMARY.md)** - How we refactored to clean architecture

## Performance
- **[Optimizations](OPTIMIZATIONS.md)** - Performance optimizations and improvements

## Usage Guides
- **[Subnet System Usage](SUBNET_USAGE.md)** - API usage and examples for subnet management

## Implementation Details
- **[Implementation Summary](IMPLEMENTATION_SUMMARY.md)** - Technical implementation details and decisions

## Directory Structure

```
docs/
├── README.md                      # This file
├── SUBNET_QUICKSTART.md           # Quick start guide
├── ARCHITECTURE.md                # Overall architecture
├── SUBNET_SYSTEM_DESIGN.md        # Subnet system design
├── SUBNET_USAGE.md                # API usage guide
├── IMPLEMENTATION_SUMMARY.md      # Implementation details
└── REFACTORING_SUMMARY.md         # Refactoring notes
```

## Documentation Guidelines

All documentation should be placed in this `docs/` directory to keep the project root clean.

### For New Features
When adding new features, create documentation following this pattern:
- `<FEATURE>_QUICKSTART.md` - Quick start guide
- `<FEATURE>_DESIGN.md` - Design decisions and architecture
- `<FEATURE>_USAGE.md` - API and usage examples

Update this README.md with links to the new documentation.
