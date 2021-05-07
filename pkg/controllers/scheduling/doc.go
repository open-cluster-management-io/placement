package scheduling

// The flow of making placement decisons is depicted as below:
// 1. A Placement is created in a working namespace;
// 2. Controller, placementDecisionCreatingController,  creates one or multiple PlacementDecisions for
//   the Placement in the same namespace;
// 3. Controller, decisionPlaceholderController, creates emtpy cluster decisions as unscheduled decisions
//   in the status of PlacementDecisions;
// 4. Controller, decisionSchedulingController, tries to find feasiable ManagedCluster for each
//   unscheduled decision in the status of PlacementDecisions, and requeues the PlacementDecision if there
//   exists any unscheduled decision for further reconciliation.
// 5. Descheduling contollers will be created in a separate package (descheduling). Each of them will handle
//   the change events of a particular resource, like ManagedCluster, and replace the corresponding scheduled
//  cluster decisions in the status of PlacementDecisions with emtpy ones.
