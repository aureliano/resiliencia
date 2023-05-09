/*
O pacote core contém as definições de tipos e funções necessárias ao funcionamento das funcionalidades
desta biblioteca. As declarações dos contratos que definem o comportamento que as métricas e as políticas
devem serguir estão neste pacote.

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
