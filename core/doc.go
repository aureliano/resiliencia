/*
The core package contains the definitions of types and functions necessary for the functionalities to work.
of this library. Contract statements that define the behavior that metrics and policies
must follow are in this package.

# Command

Command is the type that Policies use as a supplier. Indeed, it is just a pointer to an anonymous function.

# PolicySupplier

PolicySupplier is the interface that all Policies must implement. Its contract defines some obligations
that every policy must follow.

# MetricRecorder

MetricRecorder is the interface that all Metrics must implement. It defines some behaviors that metrics
are expected to be.

# Metric

Metric is the base metric recorder type. This is the one which is passed through the life cycle of an
execution chain.
*/
package core
