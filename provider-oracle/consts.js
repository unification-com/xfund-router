
const REQUEST_STATUS = {
  REQUEST_STATUS_OPEN: 1,
  REQUEST_STATUS_RECEIVED: 2,
  REQUEST_STATUS_FULFILLING: 3,
  REQUEST_STATUS_FULFILLED: 4,
  REQUEST_STATUS_CANCELLED: 5,
  REQUEST_STATUS_REJECTED: 6,
  REQUEST_STATUS_ERROR_PROCESS: 100,
  REQUEST_STATUS_ERROR_NOT_EXIST: 101,
}

module.exports = {
  REQUEST_STATUS,
}