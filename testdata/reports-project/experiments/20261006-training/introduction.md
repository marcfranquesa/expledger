# Training progress

These **illustrative metrics** show how an experiment can present its method,
related plots, and interpretation together. They are not measured results.

## Method

- Log training and validation loss every 100 steps.
- Record throughput in tokens per second at the same steps.
- Leave a validation value blank when an evaluation is unavailable.

Both charts read `results/metrics.csv`. A row keeps the plots together on a wide
screen and stacks them on a smaller screen.

```yaml
x: step
y: [train_loss, validation_loss]
```

See the [ExpLedger project](https://github.com/marcfranquesa/expledger) for the
report format and CLI.
