# TrainerV1alpha1JobStatus

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**active** | **int** | Active is the number of child Jobs with at least 1 pod in a running or pending state which are not marked for deletion. | [default to 0]
**failed** | **int** | Failed is the number of failed child Jobs. | [default to 0]
**name** | **str** | Name of the child Job. | [default to '']
**ready** | **int** | Ready is the number of child Jobs where the number of ready pods and completed pods is greater than or equal to the total expected pod count for the child Job. | [default to 0]
**succeeded** | **int** | Succeeded is the number of successfully completed child Jobs. | [default to 0]
**suspended** | **int** | Suspended is the number of child Jobs which are in a suspended state. | [default to 0]

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


