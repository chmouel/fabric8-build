package controller

import (
	"context"

	"github.com/fabric8-services/fabric8-build/app"
	"github.com/fabric8-services/fabric8-build/application"
	"github.com/fabric8-services/fabric8-build/build"
	"github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-common/httpsupport"
	"github.com/fabric8-services/fabric8-common/token"
	"github.com/goadesign/goa"
	errs "github.com/pkg/errors"
	"github.com/prometheus/common/log"
)

// PipelineEnvironmentController implements the PipelineEnvironment resource.
type PipelineEnvironmentController struct {
	*goa.Controller
	db         application.DB
	svcFactory application.ServiceFactory
}

// NewPipelineEnvironmentController creates a PipelineEnvironment controller.
func NewPipelineEnvironmentController(service *goa.Service, db application.DB, svcFactory application.ServiceFactory) *PipelineEnvironmentController {
	return &PipelineEnvironmentController{
		Controller: service.NewController("PipelineEnvironmentController"),
		db:         db,
		svcFactory: svcFactory,
	}
}

func checkAndConvertEnvironment(envs []*app.EnvironmentAttributes) (ret []build.Environment, err error) {
	//TODO: check environemnts here
	for _, env := range envs {
		ret = append(ret, build.Environment{
			EnvironmentID: env.EnvUUID,
		})
	}
	return
}

// Create runs the create action.
func (c *PipelineEnvironmentController) Create(ctx *app.CreatePipelineEnvironmentsContext) error {
	tokenMgr, err := token.ReadManagerFromContext(ctx)
	if err != nil {
		return app.JSONErrorResponse(ctx, err)
	}
	_, err = tokenMgr.Locate(ctx)
	if err != nil {
		return app.JSONErrorResponse(ctx, errors.NewUnauthorizedError(err.Error()))
	}

	reqPpl := ctx.Payload.Data
	if reqPpl == nil {
		return app.JSONErrorResponse(ctx, errors.NewBadParameterError("data", nil).Expected("not nil"))
	}

	spaceID := ctx.SpaceID
	err = c.checkSpaceExist(ctx, spaceID.String())
	if err != nil {
		return app.JSONErrorResponse(ctx, err)
	}

	newEnvs, err := checkAndConvertEnvironment(reqPpl.Environments)
	if err != nil {
		return app.JSONErrorResponse(ctx, err)
	}

	var ppl *build.Pipeline
	err = application.Transactional(c.db, func(appl application.Application) error {
		newPipeline := build.Pipeline{
			Name:        &reqPpl.Name,
			SpaceID:     &spaceID,
			Environment: newEnvs,
		}

		ppl, err = appl.Pipeline().Create(ctx, &newPipeline)
		if err != nil {
			log.Error(ctx, map[string]interface{}{"err": err},
				"failed to create pipeline: %s", newPipeline.Name)
			return errs.Wrapf(err, "failed to create pipeline: %s", *newPipeline.Name)
		}

		return nil
	})

	if err != nil {
		return app.JSONErrorResponse(ctx, err)
	}

	newEnvAttributes := []*app.EnvironmentAttributes{}
	for _, pipeline := range ppl.Environment {
		newEnvAttributes = append(newEnvAttributes, &app.EnvironmentAttributes{
			EnvUUID: pipeline.EnvironmentID,
		})
	}

	res := &app.PipelineEnvironmentSingle{
		Data: &app.PipelineEnvironments{
			ID:           &ppl.ID,
			Name:         *ppl.Name,
			Environments: newEnvAttributes,
			SpaceID:      ppl.SpaceID,
		},
	}

	ctx.ResponseData.Header().Set(
		"Location",
		httpsupport.AbsoluteURL(&goa.RequestData{Request: ctx.Request}, app.PipelineEnvironmentsHref(res.Data.ID), nil),
	)
	return ctx.Created(res)
}

// Show runs the show action.
func (c *PipelineEnvironmentController) Show(ctx *app.ShowPipelineEnvironmentsContext) error {
	spaceID := ctx.SpaceID

	ppl, err := c.db.Pipeline().Load(ctx, spaceID)
	if err != nil {
		return app.JSONErrorResponse(ctx, err)
	}

	newEnvAttributes := []*app.EnvironmentAttributes{}
	for _, pipeline := range ppl.Environment {
		newEnvAttributes = append(newEnvAttributes, &app.EnvironmentAttributes{
			EnvUUID: pipeline.EnvironmentID,
		})
	}

	res := &app.PipelineEnvironmentSingle{
		Data: &app.PipelineEnvironments{
			ID:           &ppl.ID,
			Name:         *ppl.Name,
			Environments: newEnvAttributes,
			SpaceID:      ppl.SpaceID,
		},
	}

	return ctx.OK(res)
}

func (c *PipelineEnvironmentController) checkSpaceExist(ctx context.Context, spaceID string) error {
	// TODO(chmouel): Make sure we have the rights for that space
	// TODO(chmouel): Better error reporting when NOTFound
	_, err := c.svcFactory.WITService().GetSpace(ctx, spaceID)
	if err != nil {
		return errs.Wrapf(err, "failed to get space id: %s from wit", spaceID)
	}
	return nil
}
