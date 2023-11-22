package executionvenue

import (
	"context"
	"fmt"
	api "github.com/ettec/otp-common/api/executionvenue"
	"github.com/ettec/otp-common/model"
	"github.com/ettec/otp-common/ordermanagement"
	"github.com/ettec/otp-common/staticdata"
	"github.com/google/uuid"
	"log"
	"os"
)

type orderGateway interface {
	Send(order *model.Order, listing *model.Listing) error
	Cancel(order *model.Order) error
	Modify(order *model.Order, listing *model.Listing, Quantity *model.Decimal64, Price *model.Decimal64) error
}

type orderManagerImpl struct {
	createOrderChan    chan createAndRouteOrderCmd
	cancelOrderChan    chan cancelOrderCmd
	modifyOrderChan    chan modifyOrderCmd
	setOrderStatusChan chan setOrderStatusCmd
	setOrderErrMsgChan chan setOrderErrorMsgCmd
	addExecChan        chan addExecutionCmd

	closeChan chan struct{}

	orderStore *ordermanagement.OrderCache
	gateway    orderGateway
	getListing func(ctx context.Context, listingId int32, result chan<- staticdata.ListingResult)

	errLog *log.Logger
}

func NewOrderManager(ctx context.Context, cache *ordermanagement.OrderCache, gateway orderGateway,
	getListing func(ctx context.Context, listingId int32, result chan<- staticdata.ListingResult)) *orderManagerImpl {

	om := &orderManagerImpl{
		errLog:     log.New(os.Stderr, "", log.Lshortfile|log.Ltime),
		getListing: getListing,
	}

	om.createOrderChan = make(chan createAndRouteOrderCmd, 100)
	om.cancelOrderChan = make(chan cancelOrderCmd, 100)
	om.modifyOrderChan = make(chan modifyOrderCmd, 100)
	om.setOrderStatusChan = make(chan setOrderStatusCmd, 100)
	om.setOrderErrMsgChan = make(chan setOrderErrorMsgCmd, 100)
	om.addExecChan = make(chan addExecutionCmd, 100)

	om.closeChan = make(chan struct{}, 1)

	om.orderStore = cache
	om.gateway = gateway

	go om.executeOrderCommands(ctx)

	return om
}

func (om *orderManagerImpl) executeOrderCommands(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			return
		case <-om.closeChan:
			return
		// Cancel Requests take priority over all other message types
		case oc := <-om.cancelOrderChan:
			om.executeCancelOrderCmd(ctx, oc.Params, oc.ResultChan)
		default:
			select {
			case <-ctx.Done():
				return
			case oc := <-om.cancelOrderChan:
				om.executeCancelOrderCmd(ctx, oc.Params, oc.ResultChan)
			case mp := <-om.modifyOrderChan:
				om.executeModifyOrderCmd(ctx, mp.Params, mp.ResultChan)
			case cro := <-om.createOrderChan:
				om.executeCreateAndRouteOrderCmd(ctx, cro.Params, cro.ResultChan)
			case su := <-om.setOrderStatusChan:
				om.executeSetOrderStatusCmd(ctx, su.orderId, su.status, su.ResultChan)
			case em := <-om.setOrderErrMsgChan:
				om.executeSetErrorMsg(ctx, em.orderId, em.msg, em.ResultChan)
			case tu := <-om.addExecChan:
				om.executeUpdateTradedQntCmd(ctx, tu.orderId, tu.lastPrice, tu.lastQty, tu.execId, tu.ResultChan)
			}
		}
	}

}

func (om *orderManagerImpl) Close() {
	om.closeChan <- struct{}{}
}

func (om *orderManagerImpl) SetErrorMsg(orderId string, msg string) error {
	log.Printf("updating order %v error message to %v", orderId, msg)

	resultChan := make(chan errorCmdResult)

	om.setOrderErrMsgChan <- setOrderErrorMsgCmd{
		orderId:    orderId,
		msg:        msg,
		ResultChan: resultChan,
	}

	result := <-resultChan

	return result.Error
}

func (om *orderManagerImpl) SetOrderStatus(orderId string, status model.OrderStatus) error {
	log.Printf("updating order %v status to %v", orderId, status)

	resultChan := make(chan errorCmdResult)

	om.setOrderStatusChan <- setOrderStatusCmd{
		orderId:    orderId,
		status:     status,
		ResultChan: resultChan,
	}

	result := <-resultChan

	return result.Error
}

func (om *orderManagerImpl) AddExecution(orderId string, lastPrice model.Decimal64, lastQty model.Decimal64,
	execId string) error {
	log.Printf(orderId+":adding execution for price %v and quantity %v", lastPrice, lastQty)

	resultChan := make(chan errorCmdResult)

	om.addExecChan <- addExecutionCmd{
		orderId:    orderId,
		lastPrice:  lastPrice,
		lastQty:    lastQty,
		execId:     execId,
		ResultChan: resultChan,
	}

	result := <-resultChan

	log.Printf(orderId+":update traded quantity result:%v", result)

	return result.Error
}

func (om *orderManagerImpl) CreateAndRouteOrder(params *api.CreateAndRouteOrderParams) (*api.OrderId, error) {

	resultChan := make(chan createAndRouteOrderCmdResult)

	om.createOrderChan <- createAndRouteOrderCmd{
		Params:     params,
		ResultChan: resultChan,
	}

	result := <-resultChan

	if result.Error != nil {
		return nil, result.Error
	}

	return result.OrderId, nil
}

func (om *orderManagerImpl) ModifyOrder(params *api.ModifyOrderParams) error {
	log.Printf("modifying order %v, price %v, quantity %v", params.OrderId, params.Price, params.Quantity)
	resultChan := make(chan errorCmdResult)

	om.modifyOrderChan <- modifyOrderCmd{
		Params:     params,
		ResultChan: resultChan,
	}

	result := <-resultChan

	log.Printf(params.OrderId+":modify order result: %v", result)

	return result.Error
}

func (om *orderManagerImpl) CancelOrder(params *api.CancelOrderParams) error {

	log.Print(params.OrderId + ":cancelling order")

	resultChan := make(chan errorCmdResult)

	om.cancelOrderChan <- cancelOrderCmd{
		Params:     params,
		ResultChan: resultChan,
	}

	result := <-resultChan

	log.Printf(params.OrderId+":cancel order result: %v", result)

	return result.Error
}

func (om *orderManagerImpl) executeUpdateTradedQntCmd(ctx context.Context, id string, lastPrice model.Decimal64,
	lastQty model.Decimal64,
	execId string, resultChan chan errorCmdResult) {

	order, exists, err := om.orderStore.GetOrder(id)
	if err != nil {
		resultChan <- errorCmdResult{Error: err}
		return
	}

	if !exists {
		resultChan <- errorCmdResult{Error: fmt.Errorf("update traded quantity failed, no order found for id %v", id)}
		return
	}

	err = order.AddExecution(model.Execution{
		Id:    execId,
		Price: lastPrice,
		Qty:   lastQty,
	})

	if err != nil {
		resultChan <- errorCmdResult{Error: err}
		return
	}

	err = om.orderStore.Store(ctx, order)
	resultChan <- errorCmdResult{Error: err}

}

func (om *orderManagerImpl) executeSetErrorMsg(ctx context.Context, id string, msg string, resultChan chan errorCmdResult) {

	order, exists, err := om.orderStore.GetOrder(id)
	if err != nil {
		resultChan <- errorCmdResult{Error: fmt.Errorf("failed to get order from cache %v", id)}
	}

	if !exists {
		resultChan <- errorCmdResult{Error: fmt.Errorf("set order error message failed, no order found for id %v", id)}
		return
	}

	order.ErrorMessage = msg

	err = om.orderStore.Store(ctx, order)

}

func (om *orderManagerImpl) executeSetOrderStatusCmd(ctx context.Context, id string, status model.OrderStatus,
	resultChan chan errorCmdResult) {

	order, exists, err := om.orderStore.GetOrder(id)
	if err != nil {
		resultChan <- errorCmdResult{Error: err}
		return
	}

	if !exists {
		resultChan <- errorCmdResult{Error: fmt.Errorf("set order status failed, no order found for id %v", id)}
		return
	}

	err = order.SetStatus(status)
	if err != nil {
		resultChan <- errorCmdResult{Error: err}
		return
	}

	err = om.orderStore.Store(ctx, order)

	resultChan <- errorCmdResult{Error: err}

}

func (om *orderManagerImpl) executeModifyOrderCmd(ctx context.Context, params *api.ModifyOrderParams, resultChan chan errorCmdResult) {

	order, exists, err := om.orderStore.GetOrder(params.OrderId)
	if err != nil {
		resultChan <- errorCmdResult{Error: err}
		return
	}

	if !exists {
		resultChan <- errorCmdResult{Error: fmt.Errorf("modify order failed, no order found for id %v", params.OrderId)}
		return
	}

	err = order.SetTargetStatus(model.OrderStatus_LIVE)
	if err != nil {
		resultChan <- errorCmdResult{Error: err}
		return
	}

	order.Price = params.Price
	order.Quantity = params.Quantity

	err = om.orderStore.Store(ctx, order)
	if err != nil {
		resultChan <- errorCmdResult{Error: err}
		return
	}

	listingChan := make(chan staticdata.ListingResult, 1)
	om.getListing(ctx, params.ListingId, listingChan)
	listingResult := <-listingChan
	if listingResult.Err != nil {
		resultChan <- errorCmdResult{Error: listingResult.Err}
		return
	}

	err = om.gateway.Modify(order, listingResult.Listing, params.Quantity, params.Price)

	resultChan <- errorCmdResult{Error: err}
}

func (om *orderManagerImpl) executeCancelOrderCmd(ctx context.Context,
	params *api.CancelOrderParams, resultChan chan errorCmdResult) {

	order, exists, err := om.orderStore.GetOrder(params.OrderId)
	if err != nil {
		resultChan <- errorCmdResult{Error: err}
		return
	}

	if !exists {
		resultChan <- errorCmdResult{Error: fmt.Errorf("cancel order failed, no order found for id %v", params.OrderId)}
		return
	}

	err = order.SetTargetStatus(model.OrderStatus_CANCELLED)
	if err != nil {
		resultChan <- errorCmdResult{Error: err}
		return
	}

	err = om.orderStore.Store(ctx, order)
	if err != nil {
		resultChan <- errorCmdResult{Error: err}
		return
	}

	err = om.gateway.Cancel(order)

	resultChan <- errorCmdResult{Error: err}
}

func (om *orderManagerImpl) executeCreateAndRouteOrderCmd(ctx context.Context, params *api.CreateAndRouteOrderParams,
	resultChan chan createAndRouteOrderCmdResult) {

	uniqueId, err := uuid.NewUUID()
	if err != nil {
		resultChan <- createAndRouteOrderCmdResult{
			OrderId: nil,
			Error:   fmt.Errorf("failed to create new order id: %w", err),
		}
	}

	id := uniqueId.String()

	order := model.NewOrder(id, params.OrderSide, params.Quantity,
		params.Price, params.ListingId, params.OriginatorId, params.OriginatorRef,
		params.RootOriginatorId, params.RootOriginatorRef, params.Destination)

	err = order.SetTargetStatus(model.OrderStatus_LIVE)
	if err != nil {
		om.errLog.Printf("failed to set target status;%v", err)
	}

	err = om.orderStore.Store(ctx, order)
	if err != nil {
		resultChan <- createAndRouteOrderCmdResult{
			OrderId: nil,
			Error:   err,
		}

		return
	}

	listingChan := make(chan staticdata.ListingResult, 1)
	om.getListing(ctx, params.ListingId, listingChan)

	listingResult := <-listingChan
	if listingResult.Err != nil {
		resultChan <- createAndRouteOrderCmdResult{
			OrderId: nil,
			Error:   listingResult.Err,
		}
		return
	}

	err = om.gateway.Send(order, listingResult.Listing)

	resultChan <- createAndRouteOrderCmdResult{
		OrderId: &api.OrderId{
			OrderId: order.Id,
		},
		Error: err,
	}

}

type addExecutionCmd struct {
	orderId    string
	lastPrice  model.Decimal64
	lastQty    model.Decimal64
	execId     string
	ResultChan chan errorCmdResult
}

type setOrderStatusCmd struct {
	orderId    string
	status     model.OrderStatus
	ResultChan chan errorCmdResult
}

type setOrderErrorMsgCmd struct {
	orderId    string
	msg        string
	ResultChan chan errorCmdResult
}

type createAndRouteOrderCmd struct {
	Params     *api.CreateAndRouteOrderParams
	ResultChan chan createAndRouteOrderCmdResult
}

type createAndRouteOrderCmdResult struct {
	OrderId *api.OrderId
	Error   error
}

type cancelOrderCmd struct {
	Params     *api.CancelOrderParams
	ResultChan chan errorCmdResult
}

type modifyOrderCmd struct {
	Params     *api.ModifyOrderParams
	ResultChan chan errorCmdResult
}

type errorCmdResult struct {
	Error error
}
